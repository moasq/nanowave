package config

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/moasq/nanowave/internal/agentruntime"
)

// Config holds the CLI configuration.
type Config struct {
	// RuntimeKind is the selected agent runtime.
	RuntimeKind agentruntime.Kind

	// RuntimePath is the path to the selected runtime binary.
	RuntimePath string

	// DefaultModel is the configured model for the selected runtime.
	DefaultModel string

	// NanowaveRoot is the root nanowave directory (~/.nanowave/ equivalent → ~/nanowave/).
	NanowaveRoot string

	// ProjectDir is the project catalog root (~/nanowave/projects/).
	// During a build, this is where new project folders are created.
	// After SetProject(), this points to the specific project directory.
	ProjectDir string

	// NanowaveDir is the .nanowave/ state directory for the active project.
	// Empty until a project is selected via SetProject().
	NanowaveDir string

	// Agentic enables agentic mode where the LLM drives the build via tool calling.
	Agentic bool
}

// AgenticMode returns true if agentic mode is enabled.
func (c *Config) AgenticMode() bool {
	return c != nil && c.Agentic
}

type runtimePreferences struct {
	RuntimeKind   string                              `json:"runtime_kind,omitempty"`
	Model         string                              `json:"model,omitempty"`
	DefaultModels map[string]string                   `json:"default_models,omitempty"`
	RuntimeModels map[string][]runtimeModelPreference `json:"runtime_models,omitempty"`
}

type runtimeModelPreference struct {
	ID          string `json:"id,omitempty"`
	Description string `json:"description,omitempty"`
}

func (p *runtimeModelPreference) UnmarshalJSON(data []byte) error {
	var id string
	if err := json.Unmarshal(data, &id); err == nil {
		p.ID = strings.TrimSpace(id)
		p.Description = ""
		return nil
	}

	type alias runtimeModelPreference
	var decoded alias
	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}
	p.ID = strings.TrimSpace(decoded.ID)
	p.Description = strings.TrimSpace(decoded.Description)
	return nil
}

type RuntimeStatus struct {
	Kind       agentruntime.Kind
	Display    string
	BinaryPath string
	Version    string
	Auth       *agentruntime.AuthStatus
}

// ProjectInfo holds metadata about a project in the catalog.
type ProjectInfo struct {
	Name      string
	Path      string    // full path to project dir
	CreatedAt time.Time // from project.json or dir mod time
}

// Load validates the environment and returns a Config.
// ProjectDir is set to ~/nanowave/projects/ (the catalog root).
// NanowaveDir is empty until a project is selected via SetProject().
func Load() (*Config, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("failed to get home directory: %w", err)
	}

	nanowaveRoot := filepath.Join(home, "nanowave")
	projectDir := filepath.Join(nanowaveRoot, "projects")

	// Create the catalog directory if needed
	if err := os.MkdirAll(projectDir, 0o755); err != nil {
		return nil, fmt.Errorf("failed to create project catalog: %w", err)
	}

	prefs := loadRuntimePreferences(nanowaveRoot)
	runtimeKind := agentruntime.NormalizeKind(prefs.RuntimeKind)
	if runtimeKind == "" {
		if detectedKind, _, ok := agentruntime.FirstInstalledKind(); ok {
			runtimeKind = detectedKind
		} else {
			runtimeKind = agentruntime.KindClaude
		}
	}

	runtimePath, _ := agentruntime.FindBinary(runtimeKind)

	return &Config{
		RuntimeKind:  runtimeKind,
		RuntimePath:  runtimePath,
		DefaultModel: prefs.defaultModelFor(runtimeKind),
		NanowaveRoot: nanowaveRoot,
		ProjectDir:   projectDir,
		NanowaveDir:  "", // set via SetProject()
	}, nil
}

// SetProject switches config to point at a specific project directory.
// projectPath should be the full path (e.g., ~/nanowave/projects/HabitGrid).
func (c *Config) SetProject(projectPath string) {
	c.ProjectDir = projectPath
	c.NanowaveDir = filepath.Join(projectPath, ".nanowave")
}

// ListProjects scans the catalog for valid projects (dirs with .nanowave/project.json).
func (c *Config) ListProjects() []ProjectInfo {
	catalogRoot := c.CatalogRoot()

	entries, err := os.ReadDir(catalogRoot)
	if err != nil {
		return nil
	}

	var projects []ProjectInfo
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		projDir := filepath.Join(catalogRoot, entry.Name())
		projectJSON := filepath.Join(projDir, ".nanowave", "project.json")
		info, err := os.Stat(projectJSON)
		if err != nil {
			continue
		}
		projects = append(projects, ProjectInfo{
			Name:      entry.Name(),
			Path:      projDir,
			CreatedAt: info.ModTime(),
		})
	}

	// Sort by most recently modified first
	sort.Slice(projects, func(i, j int) bool {
		return projects[i].CreatedAt.After(projects[j].CreatedAt)
	})

	return projects
}

// CatalogRoot returns the project catalog root (~/nanowave/projects/).
// This is the original ProjectDir before SetProject() is called.
func (c *Config) CatalogRoot() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, "nanowave", "projects")
}

// EnsureNanowaveDir creates the .nanowave/ directory if it doesn't exist.
func (c *Config) EnsureNanowaveDir() error {
	if c.NanowaveDir == "" {
		return fmt.Errorf("no project selected")
	}
	return os.MkdirAll(c.NanowaveDir, 0o755)
}

// HasProject returns true if a .nanowave/ directory exists with a project.json.
func (c *Config) HasProject() bool {
	if c.NanowaveDir == "" {
		return false
	}
	_, err := os.Stat(filepath.Join(c.NanowaveDir, "project.json"))
	return err == nil
}

func findClaude() (string, error) {
	path, err := exec.LookPath("claude")
	if err != nil {
		return "", err
	}
	return path, nil
}

func preferencesPath(root string) string {
	return filepath.Join(root, "config.json")
}

func loadRuntimePreferences(root string) runtimePreferences {
	data, err := os.ReadFile(preferencesPath(root))
	if err != nil {
		return runtimePreferences{}
	}
	var prefs runtimePreferences
	if err := json.Unmarshal(data, &prefs); err != nil {
		return runtimePreferences{}
	}
	return prefs
}

func (p runtimePreferences) defaultModelFor(kind agentruntime.Kind) string {
	normalized := agentruntime.NormalizeKind(string(kind))
	if normalized == "" {
		return ""
	}
	if len(p.DefaultModels) > 0 {
		if model := strings.TrimSpace(p.DefaultModels[string(normalized)]); model != "" {
			return model
		}
	}
	if agentruntime.NormalizeKind(p.RuntimeKind) == normalized {
		return strings.TrimSpace(p.Model)
	}
	return ""
}

func (p *runtimePreferences) setDefaultModel(kind agentruntime.Kind, model string) {
	normalized := agentruntime.NormalizeKind(string(kind))
	model = strings.TrimSpace(model)
	p.RuntimeKind = string(normalized)
	p.Model = model
	if p.DefaultModels == nil {
		p.DefaultModels = make(map[string]string)
	}
	if model == "" {
		delete(p.DefaultModels, string(normalized))
		return
	}
	p.DefaultModels[string(normalized)] = model
}

func (p runtimePreferences) configuredModelsFor(kind agentruntime.Kind) []agentruntime.ModelOption {
	normalized := agentruntime.NormalizeKind(string(kind))
	var configured []agentruntime.ModelOption
	for _, option := range p.RuntimeModels[string(normalized)] {
		id := strings.TrimSpace(option.ID)
		if id == "" {
			continue
		}
		configured = append(configured, agentruntime.ModelOption{
			ID:          id,
			Description: strings.TrimSpace(option.Description),
		})
	}
	if model := p.defaultModelFor(normalized); model != "" {
		configured = append(configured, agentruntime.ModelOption{
			ID:          model,
			Description: "Selected model",
		})
	}
	return agentruntime.MergeModelOptions(configured)
}

func (c *Config) DefaultModelForRuntime(kind agentruntime.Kind) string {
	if c == nil {
		return ""
	}
	prefs := loadRuntimePreferences(c.NanowaveRoot)
	return prefs.defaultModelFor(kind)
}

func (c *Config) RuntimeModelOptions(kind agentruntime.Kind, discovered []agentruntime.ModelOption) []agentruntime.ModelOption {
	if c == nil {
		return agentruntime.MergeModelOptions(discovered)
	}
	prefs := loadRuntimePreferences(c.NanowaveRoot)
	configured := prefs.configuredModelsFor(kind)
	return agentruntime.MergeModelOptions(configured, discovered)
}

func (c *Config) SaveRuntimePreferences(kind agentruntime.Kind, model string) error {
	prefs := loadRuntimePreferences(c.NanowaveRoot)
	prefs.setDefaultModel(kind, model)
	data, err := json.MarshalIndent(prefs, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal runtime preferences: %w", err)
	}
	if err := os.MkdirAll(c.NanowaveRoot, 0o755); err != nil {
		return fmt.Errorf("failed to create nanowave root: %w", err)
	}
	if err := os.WriteFile(preferencesPath(c.NanowaveRoot), data, 0o644); err != nil {
		return fmt.Errorf("failed to write runtime preferences: %w", err)
	}
	c.RuntimeKind = kind
	c.DefaultModel = prefs.defaultModelFor(kind)
	if path, err := agentruntime.FindBinary(kind); err == nil {
		c.RuntimePath = path
	}
	return nil
}

func CheckRuntime(kind agentruntime.Kind) bool {
	_, err := agentruntime.FindBinary(kind)
	return err == nil
}

func RuntimeVersion(kind agentruntime.Kind, runtimePath string) string {
	runtimePath = strings.TrimSpace(runtimePath)
	if runtimePath == "" {
		return ""
	}
	runtimeClient, err := agentruntime.New(kind, runtimePath)
	if err != nil {
		return ""
	}
	return runtimeClient.Version()
}

func RuntimeAuthStatus(kind agentruntime.Kind, runtimePath string) *agentruntime.AuthStatus {
	runtimePath = strings.TrimSpace(runtimePath)
	if runtimePath == "" {
		return nil
	}
	runtimeClient, err := agentruntime.New(kind, runtimePath)
	if err != nil {
		return nil
	}
	return runtimeClient.AuthStatus()
}

func DetectInstalledRuntimes() []RuntimeStatus {
	var statuses []RuntimeStatus
	for _, kind := range agentruntime.SupportedKinds() {
		path, err := agentruntime.FindBinary(kind)
		if err != nil || path == "" {
			continue
		}
		desc := agentruntime.DescriptorForKind(kind)
		statuses = append(statuses, RuntimeStatus{
			Kind:       kind,
			Display:    desc.DisplayName,
			BinaryPath: path,
			Version:    RuntimeVersion(kind, path),
			Auth:       RuntimeAuthStatus(kind, path),
		})
	}
	return statuses
}

// CheckXcode returns true if the full Xcode IDE is installed (not just CLT).
func CheckXcode() bool {
	out, err := exec.Command("xcode-select", "-p").Output()
	if err != nil {
		return false
	}
	path := strings.TrimSpace(string(out))
	// xcode-select -p returns /Applications/Xcode.app/... for full Xcode
	// or /Library/Developer/CommandLineTools for CLT only
	return strings.Contains(path, "Xcode.app")
}

// CheckXcodeCLT returns true if Xcode Command Line Tools are installed.
func CheckXcodeCLT() bool {
	cmd := exec.Command("xcode-select", "-p")
	return cmd.Run() == nil
}

// CheckSimulator returns true if an iOS Simulator runtime is available.
func CheckSimulator() bool {
	out, err := exec.Command("xcrun", "simctl", "list", "runtimes", "--json").Output()
	if err != nil {
		return false
	}
	// Quick check: if output contains "iOS" there's at least one runtime
	return strings.Contains(string(out), "iOS")
}

// CheckXcodegen returns true if xcodegen is installed.
func CheckXcodegen() bool {
	_, err := exec.LookPath("xcodegen")
	return err == nil
}

// CheckSupabaseCLI returns true if the Supabase CLI is installed.
func CheckSupabaseCLI() bool {
	_, err := exec.LookPath("supabase")
	return err == nil
}

// EnsureSupabaseCLI checks if the Supabase CLI is installed and installs it
// via Homebrew if missing. Returns true if the CLI is available after the check.
func EnsureSupabaseCLI(printFn func(level, msg string)) bool {
	if CheckSupabaseCLI() {
		return true
	}
	printFn("warning", "Supabase CLI not found — installing...")
	brewPath, err := exec.LookPath("brew")
	if err != nil || brewPath == "" {
		printFn("error", "Homebrew not found — install Supabase CLI manually: brew install supabase/tap/supabase")
		return false
	}
	cmd := exec.Command("brew", "install", "supabase/tap/supabase")
	if err := cmd.Run(); err != nil {
		printFn("error", fmt.Sprintf("Failed to install Supabase CLI: %v", err))
		printFn("info", "Install manually: brew install supabase/tap/supabase")
		return false
	}
	printFn("success", "Supabase CLI installed")
	return true
}

// ClaudeAuthStatus holds the user's Claude authentication state.
type ClaudeAuthStatus struct {
	LoggedIn         bool   `json:"loggedIn"`
	Email            string `json:"email"`
	SubscriptionType string `json:"subscriptionType"` // "free", "pro", "max"
	AuthMethod       string `json:"authMethod"`       // "claude.ai", "api_key"
}

// CheckClaudeAuth checks whether the user is authenticated with Claude Code.
func CheckClaudeAuth(claudePath string) *ClaudeAuthStatus {
	cmd := exec.Command(claudePath, "auth", "status", "--json")
	out, err := cmd.Output()
	if err != nil {
		return nil
	}

	// claude auth status --json returns a flat object:
	// {"loggedIn":true,"authMethod":"claude.ai","email":"...","subscriptionType":"max",...}
	var raw struct {
		LoggedIn         bool   `json:"loggedIn"`
		Email            string `json:"email"`
		SubscriptionType string `json:"subscriptionType"`
		AuthMethod       string `json:"authMethod"`
	}
	if err := json.Unmarshal(out, &raw); err != nil {
		// Fallback: check if output suggests logged in
		s := strings.TrimSpace(string(out))
		if strings.Contains(s, "loggedIn") || strings.Contains(s, "Logged in") {
			return &ClaudeAuthStatus{LoggedIn: true}
		}
		return nil
	}

	if !raw.LoggedIn {
		return &ClaudeAuthStatus{LoggedIn: false}
	}

	return &ClaudeAuthStatus{
		LoggedIn:         true,
		Email:            raw.Email,
		SubscriptionType: raw.SubscriptionType,
		AuthMethod:       raw.AuthMethod,
	}
}

// CheckClaude returns true if Claude Code CLI is installed.
func CheckClaude() bool {
	_, err := exec.LookPath("claude")
	return err == nil
}

// ClaudeVersion returns the installed Claude Code version.
func ClaudeVersion(claudePath string) string {
	cmd := exec.Command(claudePath, "--version")
	out, err := cmd.Output()
	if err != nil {
		return "unknown"
	}
	return strings.TrimSpace(string(out))
}
