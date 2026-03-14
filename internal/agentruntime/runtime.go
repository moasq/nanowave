package agentruntime

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/moasq/nanowave/internal/claude"
)

type Kind string

const (
	KindClaude   Kind = "claude"
	KindCodex    Kind = "codex"
	KindOpenCode Kind = "opencode"
)

type Phase string

const (
	PhaseIntent   Phase = "intent"
	PhaseAnalyze  Phase = "analyze"
	PhasePlan     Phase = "plan"
	PhaseBuild    Phase = "build"
	PhaseFix      Phase = "fix"
	PhaseQuestion Phase = "question"
)

type Usage = claude.Usage
type Response = claude.Response
type StreamEvent = claude.StreamEvent
type GenerateOpts = claude.GenerateOpts
type InteractiveOpts = claude.InteractiveOpts

type Runtime interface {
	Kind() Kind
	DisplayName() string
	DefaultModel(phase Phase) string
	SuggestedModels() []ModelOption
	SupportsInteractive() bool
	GenerateStreaming(ctx context.Context, userMessage string, opts GenerateOpts, onEvent func(StreamEvent)) (*Response, error)
	Generate(ctx context.Context, userMessage string, opts GenerateOpts) (*Response, error)
	RunInteractive(ctx context.Context, prompt string, opts InteractiveOpts, onEvent func(StreamEvent), onQuestion func(question string) string) (*Response, error)
}

type ModelOption struct {
	ID          string
	Description string
}

type AuthStatus struct {
	LoggedIn bool
	Email    string
	Plan     string
	Detail   string
}

type Descriptor struct {
	Kind                Kind
	DisplayName         string
	BinaryName          string
	InstallCommand      string
	SetupHint           string
	SupportsInteractive bool
}

func AllDescriptors() []Descriptor {
	return []Descriptor{
		{
			Kind:                KindClaude,
			DisplayName:         "Claude Code",
			BinaryName:          "claude",
			InstallCommand:      "curl -fsSL https://claude.ai/install.sh | bash",
			SetupHint:           "Run `claude auth login` after install.",
			SupportsInteractive: true,
		},
		{
			Kind:                KindCodex,
			DisplayName:         "Codex",
			BinaryName:          "codex",
			InstallCommand:      "npm install -g @openai/codex",
			SetupHint:           "Run `codex login` after install.",
			SupportsInteractive: false,
		},
		{
			Kind:                KindOpenCode,
			DisplayName:         "OpenCode",
			BinaryName:          "opencode",
			InstallCommand:      "curl -fsSL https://opencode.ai/install | bash",
			SetupHint:           "Run `opencode auth login` after install, then choose a provider/model.",
			SupportsInteractive: false,
		},
	}
}

func NormalizeKind(raw string) Kind {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "", "claude", "claude-code":
		return KindClaude
	case "codex", "openai-codex":
		return KindCodex
	case "opencode", "open-code":
		return KindOpenCode
	default:
		return Kind(strings.ToLower(strings.TrimSpace(raw)))
	}
}

func DescriptorForKind(kind Kind) Descriptor {
	normalized := NormalizeKind(string(kind))
	for _, desc := range AllDescriptors() {
		if desc.Kind == normalized {
			return desc
		}
	}
	return Descriptor{
		Kind:                normalized,
		DisplayName:         strings.Title(string(normalized)),
		BinaryName:          string(normalized),
		SupportsInteractive: false,
	}
}

func SupportedKinds() []Kind {
	return []Kind{KindClaude, KindCodex, KindOpenCode}
}

func FindBinary(kind Kind) (string, error) {
	name := strings.TrimSpace(DescriptorForKind(kind).BinaryName)
	if name == "" {
		return "", exec.ErrNotFound
	}
	if path, err := exec.LookPath(name); err == nil && path != "" {
		return path, nil
	}
	return findBinaryInDirs(name, candidateBinaryDirs())
}

func FirstInstalledKind() (Kind, string, bool) {
	for _, kind := range SupportedKinds() {
		path, err := FindBinary(kind)
		if err == nil && path != "" {
			return kind, path, true
		}
	}
	return "", "", false
}

func New(kind Kind, path string) (Runtime, error) {
	switch NormalizeKind(string(kind)) {
	case KindClaude:
		return NewClaude(path), nil
	case KindCodex:
		return NewCodex(path), nil
	case KindOpenCode:
		return NewOpenCode(path), nil
	default:
		return nil, exec.ErrNotFound
	}
}

func MergeModelOptions(groups ...[]ModelOption) []ModelOption {
	seen := make(map[string]int)
	merged := make([]ModelOption, 0)
	for _, group := range groups {
		for _, option := range group {
			id := strings.TrimSpace(option.ID)
			if id == "" {
				continue
			}
			desc := strings.TrimSpace(option.Description)
			if idx, ok := seen[id]; ok {
				if merged[idx].Description == "" && desc != "" {
					merged[idx].Description = desc
				}
				continue
			}
			merged = append(merged, ModelOption{
				ID:          id,
				Description: desc,
			})
			seen[id] = len(merged) - 1
		}
	}
	return merged
}

func summarizeCommandOutput(text string, prefer ...string) string {
	var last string
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		last = line
		lower := strings.ToLower(line)
		for _, token := range prefer {
			if strings.Contains(lower, strings.ToLower(token)) {
				return line
			}
		}
	}
	return last
}

func findBinaryInDirs(name string, dirs []string) (string, error) {
	for _, dir := range dirs {
		candidate := filepath.Join(dir, name)
		info, err := os.Stat(candidate)
		if err != nil || info.IsDir() {
			continue
		}
		if info.Mode()&0o111 == 0 {
			continue
		}
		return candidate, nil
	}
	return "", exec.ErrNotFound
}

func candidateBinaryDirs() []string {
	seen := make(map[string]struct{})
	var dirs []string
	addDir := func(raw string) {
		raw = strings.TrimSpace(expandUserPath(raw))
		if raw == "" {
			return
		}
		raw = filepath.Clean(raw)
		if _, ok := seen[raw]; ok {
			return
		}
		seen[raw] = struct{}{}
		dirs = append(dirs, raw)
	}

	for _, dir := range strings.Split(os.Getenv("PATH"), string(os.PathListSeparator)) {
		addDir(dir)
	}

	if home, err := os.UserHomeDir(); err == nil && home != "" {
		addDir(filepath.Join(home, ".local", "bin"))
		addDir(filepath.Join(home, "bin"))
		addDir(filepath.Join(home, ".npm-global", "bin"))
		addDir(filepath.Join(home, ".cargo", "bin"))
		addDir(filepath.Join(home, "go", "bin"))
		addDir(filepath.Join(home, ".opencode", "bin"))
	}

	addDir("/opt/homebrew/bin")
	addDir("/usr/local/bin")

	return dirs
}

func expandUserPath(raw string) string {
	if raw == "~" || strings.HasPrefix(raw, "~/") {
		home, err := os.UserHomeDir()
		if err != nil || home == "" {
			return raw
		}
		if raw == "~" {
			return home
		}
		return filepath.Join(home, strings.TrimPrefix(raw, "~/"))
	}
	return raw
}
