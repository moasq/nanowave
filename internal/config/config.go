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
)

// Config holds the CLI configuration.
type Config struct {
	// ClaudePath is the path to the claude binary.
	ClaudePath string

	// NanowaveRoot is the root nanowave directory (~/.nanowave/ equivalent → ~/nanowave/).
	NanowaveRoot string

	// ProjectDir is the project catalog root (~/nanowave/projects/).
	// During a build, this is where new project folders are created.
	// After SetProject(), this points to the specific project directory.
	ProjectDir string

	// NanowaveDir is the .nanowave/ state directory for the active project.
	// Empty until a project is selected via SetProject().
	NanowaveDir string
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
	claudePath, err := findClaude()
	if err != nil {
		return nil, fmt.Errorf("claude Code CLI not found: %w\nInstall: curl -fsSL https://claude.ai/install.sh | bash", err)
	}

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

	return &Config{
		ClaudePath:   claudePath,
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

