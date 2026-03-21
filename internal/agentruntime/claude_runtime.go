package agentruntime

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

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
	return claudeModelCatalog().Default
}

func (r *ClaudeRuntime) SuggestedModels() []ModelOption {
	return claudeModelCatalog().Models
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

func claudeModelCatalog() discoveredModelCatalog {
	return mergeDiscoveredModelCatalogs(
		discoverJSONModelCatalog(claudeSettingsPaths()...),
		discoverClaudeStatsCatalog(claudeStatsCachePath()),
	)
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
