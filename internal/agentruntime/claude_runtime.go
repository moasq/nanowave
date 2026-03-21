package agentruntime

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/moasq/nanowave/internal/claude"
)

type ClaudeRuntime struct {
	path   string
	client *claude.Client
}

func NewClaude(path string) *ClaudeRuntime {
	return &ClaudeRuntime{
		path:   path,
		client: claude.NewClient(path),
	}
}

func (r *ClaudeRuntime) Kind() Kind {
	return KindClaude
}

func (r *ClaudeRuntime) DisplayName() string {
	return DescriptorForKind(KindClaude).DisplayName
}

func (r *ClaudeRuntime) Descriptor() Descriptor {
	return DescriptorForKind(KindClaude)
}

func (r *ClaudeRuntime) BinaryPath() string {
	return r.path
}

func (r *ClaudeRuntime) Version() string {
	return ClaudeVersion(r.path)
}

func (r *ClaudeRuntime) AuthStatus() *AuthStatus {
	return CheckClaudeAuth(r.path)
}

func (r *ClaudeRuntime) DefaultModel(_ Phase) string {
	return r.modelCatalog().Default
}

func (r *ClaudeRuntime) SuggestedModels() []ModelOption {
	return r.modelCatalog().Models
}

func (r *ClaudeRuntime) SupportsInteractive() bool {
	return true
}

func (r *ClaudeRuntime) Generate(ctx context.Context, userMessage string, opts GenerateOpts) (*Response, error) {
	return r.client.Generate(ctx, userMessage, opts)
}

func (r *ClaudeRuntime) GenerateStreaming(ctx context.Context, userMessage string, opts GenerateOpts, onEvent func(StreamEvent)) (*Response, error) {
	return r.client.GenerateStreaming(ctx, userMessage, opts, onEvent)
}

func (r *ClaudeRuntime) RunInteractive(ctx context.Context, prompt string, opts InteractiveOpts, onEvent func(StreamEvent), onQuestion func(question string) string) (*Response, error) {
	return r.client.RunInteractive(ctx, prompt, opts, onEvent, onQuestion)
}

func (r *ClaudeRuntime) modelCatalog() discoveredModelCatalog {
	return claudeModelCatalog(r.path)
}

type claudeStatsCache struct {
	DailyStats []struct {
		TokensByModel map[string]int64 `json:"tokensByModel"`
	} `json:"dailyStats"`
	ModelUsage map[string]struct {
		InputTokens              int64 `json:"inputTokens"`
		OutputTokens             int64 `json:"outputTokens"`
		CacheReadInputTokens     int64 `json:"cacheReadInputTokens"`
		CacheCreationInputTokens int64 `json:"cacheCreationInputTokens"`
	} `json:"modelUsage"`
}

func claudeModelCatalog(path string) discoveredModelCatalog {
	catalog := mergeDiscoveredModelCatalogs(
		discoverJSONModelCatalog(claudeSettingsPaths()...),
		discoverClaudeStatsCatalog(claudeStatsCachePath()),
	)
	if len(catalog.Models) == 0 {
		catalog = mergeDiscoveredModelCatalogs(catalog, discoverClaudeTelemetryCatalog(claudeTelemetryPaths(24)...))
	}
	if len(catalog.Models) == 0 {
		catalog = mergeDiscoveredModelCatalogs(catalog, discoverClaudeHelpCatalog(path))
	}
	if catalog.Default == "" {
		catalog.Default = "sonnet"
	}
	if len(catalog.Models) == 0 {
		catalog.Models = []ModelOption{{
			ID:          catalog.Default,
			Description: "Claude Code alias",
		}}
	}
	return catalog
}

func claudeSettingsPaths() []string {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return nil
	}
	return existingPaths(
		filepath.Join(home, ".claude", "settings.json"),
		filepath.Join(home, ".claude", "settings.local.json"),
	)
}

func claudeStatsCachePath() string {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return ""
	}
	return filepath.Join(home, ".claude", "stats-cache.json")
}

func claudeTelemetryPaths(limit int) []string {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return nil
	}
	dir := filepath.Join(home, ".claude", "telemetry")
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}

	type telemetryFile struct {
		path    string
		modTime time.Time
	}

	files := make([]telemetryFile, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			continue
		}
		files = append(files, telemetryFile{
			path:    filepath.Join(dir, entry.Name()),
			modTime: info.ModTime(),
		})
	}
	sort.Slice(files, func(i, j int) bool {
		return files[i].modTime.After(files[j].modTime)
	})
	if limit <= 0 || limit > len(files) {
		limit = len(files)
	}
	paths := make([]string, 0, limit)
	for _, file := range files[:limit] {
		paths = append(paths, file.path)
	}
	return paths
}

func discoverClaudeStatsCatalog(path string) discoveredModelCatalog {
	path = strings.TrimSpace(expandUserPath(path))
	if path == "" {
		return discoveredModelCatalog{}
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return discoveredModelCatalog{}
	}
	var cache claudeStatsCache
	if err := json.Unmarshal(data, &cache); err != nil {
		return discoveredModelCatalog{}
	}

	recent := rankedModelOptionsFromCounts(nil, "Recently used in Claude Code")
	if count := len(cache.DailyStats); count > 0 {
		recent = rankedModelOptionsFromCounts(cache.DailyStats[count-1].TokensByModel, "Recently used in Claude Code")
	}

	usageTotals := make(map[string]int64, len(cache.ModelUsage))
	for modelID, usage := range cache.ModelUsage {
		usageTotals[modelID] = usage.InputTokens + usage.OutputTokens + usage.CacheReadInputTokens + usage.CacheCreationInputTokens
	}
	usage := rankedModelOptionsFromCounts(usageTotals, "Observed in Claude Code stats")

	defaultModel := ""
	if len(recent) > 0 {
		defaultModel = recent[0].ID
	} else if len(usage) > 0 {
		defaultModel = usage[0].ID
	}

	return discoveredModelCatalog{
		Default: defaultModel,
		Models:  MergeModelOptions(recent, usage),
	}
}

var claudeModelIDPattern = regexp.MustCompile(`\bclaude-[a-z0-9.-]+\b`)

func discoverClaudeTelemetryCatalog(paths ...string) discoveredModelCatalog {
	counts := make(map[string]int64)
	for _, path := range paths {
		path = strings.TrimSpace(expandUserPath(path))
		if path == "" {
			continue
		}
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		for _, modelID := range claudeModelIDPattern.FindAllString(strings.ToLower(string(data)), -1) {
			if !looksLikeClaudeModelID(modelID) {
				continue
			}
			counts[modelID]++
		}
	}
	models := rankedModelOptionsFromCounts(counts, "Observed in Claude Code telemetry")
	defaultModel := ""
	if len(models) > 0 {
		defaultModel = models[0].ID
	}
	return discoveredModelCatalog{
		Default: defaultModel,
		Models:  models,
	}
}

func discoverClaudeHelpCatalog(path string) discoveredModelCatalog {
	path = strings.TrimSpace(path)
	if path == "" {
		return discoveredModelCatalog{}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	out, err := exec.CommandContext(ctx, path, "--help").Output()
	if err != nil {
		return discoveredModelCatalog{}
	}

	helpText := string(out)
	var models []ModelOption
	for _, alias := range []string{"sonnet", "opus", "haiku"} {
		if strings.Contains(helpText, "'"+alias+"'") {
			models = append(models, ModelOption{
				ID:          alias,
				Description: "Claude Code alias",
			})
		}
	}
	for _, modelID := range claudeModelIDPattern.FindAllString(strings.ToLower(helpText), -1) {
		if !looksLikeClaudeModelID(modelID) {
			continue
		}
		models = append(models, ModelOption{
			ID:          modelID,
			Description: "Discovered from Claude CLI help",
		})
	}

	defaultModel := ""
	for _, model := range models {
		if model.ID == "sonnet" {
			defaultModel = model.ID
			break
		}
		if defaultModel == "" {
			defaultModel = model.ID
		}
	}
	return discoveredModelCatalog{
		Default: defaultModel,
		Models:  MergeModelOptions(models),
	}
}

func looksLikeClaudeModelID(raw string) bool {
	modelID := strings.TrimSpace(strings.ToLower(raw))
	if !strings.HasPrefix(modelID, "claude-") || strings.HasPrefix(modelID, "claude-code-") {
		return false
	}
	hasDigit := false
	for _, r := range modelID {
		if r >= '0' && r <= '9' {
			hasDigit = true
			break
		}
	}
	return hasDigit && strings.Count(modelID, "-") >= 2
}

func rankedModelOptionsFromCounts(counts map[string]int64, description string) []ModelOption {
	if len(counts) == 0 {
		return nil
	}
	type countedModel struct {
		id    string
		count int64
	}
	ranked := make([]countedModel, 0, len(counts))
	for id, count := range counts {
		id = strings.TrimSpace(id)
		if id == "" {
			continue
		}
		ranked = append(ranked, countedModel{id: id, count: count})
	}
	sort.Slice(ranked, func(i, j int) bool {
		if ranked[i].count == ranked[j].count {
			return ranked[i].id < ranked[j].id
		}
		return ranked[i].count > ranked[j].count
	})

	models := make([]ModelOption, 0, len(ranked))
	for _, item := range ranked {
		models = append(models, ModelOption{
			ID:          item.id,
			Description: description,
		})
	}
	return models
}

func CheckClaudeAuth(path string) *AuthStatus {
	if strings.TrimSpace(path) == "" {
		return nil
	}
	cmd := exec.Command(path, "auth", "status", "--json")
	out, err := cmd.Output()
	if err != nil {
		return nil
	}

	var raw struct {
		LoggedIn         bool   `json:"loggedIn"`
		Email            string `json:"email"`
		SubscriptionType string `json:"subscriptionType"`
		AuthMethod       string `json:"authMethod"`
	}
	if err := json.Unmarshal(out, &raw); err != nil {
		return nil
	}

	return &AuthStatus{
		LoggedIn: raw.LoggedIn,
		Email:    strings.TrimSpace(raw.Email),
		Plan:     strings.TrimSpace(raw.SubscriptionType),
		Detail:   strings.TrimSpace(raw.AuthMethod),
	}
}

func ClaudeVersion(path string) string {
	if strings.TrimSpace(path) == "" {
		return ""
	}
	cmd := exec.Command(path, "--version")
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}
