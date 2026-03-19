package hooks

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"gopkg.in/yaml.v3"
)

// Config represents the top-level hooks.yaml configuration.
type Config struct {
	Version   int                        `yaml:"version"`
	Notifiers map[string]NotifierConfig  `yaml:"notifiers"`
	Defaults  DefaultsConfig             `yaml:"defaults"`
	Hooks     map[string][]HookDefinition `yaml:"hooks"`
}

// NotifierConfig represents a notifier service configuration.
type NotifierConfig struct {
	Enabled          bool   `yaml:"enabled"`
	BotTokenEnv      string `yaml:"bot_token_env"`
	BotTokenKeychain string `yaml:"bot_token_keychain"`
	ChatID           string `yaml:"chat_id"`
	WebhookURLEnv    string `yaml:"webhook_url_env"`
}

// DefaultsConfig holds default settings for hook execution.
type DefaultsConfig struct {
	Timeout         string `yaml:"timeout"`
	ContinueOnError bool   `yaml:"continue_on_error"`
	LogDir          string `yaml:"log_dir"`
}

// HookDefinition defines a single hook action.
type HookDefinition struct {
	Name     string `yaml:"name"`
	Run      string `yaml:"run"`
	Notify   string `yaml:"notify"`
	Template string `yaml:"template"`
	When     string `yaml:"when"` // "always", "success", "failure"
	Timeout  string `yaml:"timeout"`
}

// TimeoutDuration returns the parsed timeout for a hook, falling back to the config default.
func (h *HookDefinition) TimeoutDuration(defaults DefaultsConfig) time.Duration {
	if h.Timeout != "" {
		if d, err := time.ParseDuration(h.Timeout); err == nil {
			return d
		}
	}
	if defaults.Timeout != "" {
		if d, err := time.ParseDuration(defaults.Timeout); err == nil {
			return d
		}
	}
	return 30 * time.Second
}

var (
	cachedConfig *Config
	configOnce   sync.Once
	configErr    error
)

// ResetConfigCache clears the cached config (useful for testing or re-reading).
func ResetConfigCache() {
	configOnce = sync.Once{}
	cachedConfig = nil
	configErr = nil
}

// LoadConfig loads and merges hooks configuration from global and project paths.
// Returns nil config (no error) if no config files exist.
func LoadConfig() (*Config, error) {
	configOnce.Do(func() {
		cachedConfig, configErr = loadAndMergeConfig()
	})
	return cachedConfig, configErr
}

// LoadConfigFromPath loads a single config file (used for validation).
func LoadConfigFromPath(path string) (*Config, error) {
	return parseConfigFile(path)
}

func loadAndMergeConfig() (*Config, error) {
	globalPath := globalConfigPath()
	projectPath := projectConfigPath()

	globalCfg, globalErr := parseConfigFile(globalPath)
	projectCfg, projectErr := parseConfigFile(projectPath)

	// Both missing is fine, just return nil
	if os.IsNotExist(globalErr) && os.IsNotExist(projectErr) {
		return nil, nil
	}

	// Real errors (not "not found") are returned
	if globalErr != nil && !os.IsNotExist(globalErr) {
		return nil, fmt.Errorf("hooks: global config %s: %w", globalPath, globalErr)
	}
	if projectErr != nil && !os.IsNotExist(projectErr) {
		return nil, fmt.Errorf("hooks: project config %s: %w", projectPath, projectErr)
	}

	// Only global
	if globalCfg != nil && projectCfg == nil {
		return globalCfg, nil
	}
	// Only project
	if globalCfg == nil && projectCfg != nil {
		return projectCfg, nil
	}

	// Merge: project overrides/extends global
	return mergeConfigs(globalCfg, projectCfg), nil
}

func mergeConfigs(global, project *Config) *Config {
	merged := &Config{
		Version:   global.Version,
		Notifiers: make(map[string]NotifierConfig),
		Defaults:  global.Defaults,
		Hooks:     make(map[string][]HookDefinition),
	}

	// Copy global notifiers
	for k, v := range global.Notifiers {
		merged.Notifiers[k] = v
	}
	// Project notifiers override
	for k, v := range project.Notifiers {
		merged.Notifiers[k] = v
	}

	// Project defaults override if set
	if project.Defaults.Timeout != "" {
		merged.Defaults.Timeout = project.Defaults.Timeout
	}
	if project.Defaults.LogDir != "" {
		merged.Defaults.LogDir = project.Defaults.LogDir
	}
	// ContinueOnError: project always wins
	merged.Defaults.ContinueOnError = project.Defaults.ContinueOnError

	// Merge hooks: project hooks ADD to global; same name overrides
	for event, hooks := range global.Hooks {
		merged.Hooks[event] = append(merged.Hooks[event], hooks...)
	}
	for event, projectHooks := range project.Hooks {
		existing := merged.Hooks[event]
		for _, ph := range projectHooks {
			replaced := false
			for i, eh := range existing {
				if eh.Name == ph.Name {
					existing[i] = ph
					replaced = true
					break
				}
			}
			if !replaced {
				existing = append(existing, ph)
			}
		}
		merged.Hooks[event] = existing
	}

	return merged
}

func parseConfigFile(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parse error: %w", err)
	}
	return &cfg, nil
}

func globalConfigPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".nanowave", "hooks.yaml")
}

func projectConfigPath() string {
	return filepath.Join(".nanowave", "hooks.yaml")
}

// expandPath expands ~ to the user home directory.
func expandPath(path string) string {
	if strings.HasPrefix(path, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return path
		}
		return filepath.Join(home, path[2:])
	}
	return path
}

// ResolveLogDir returns the expanded log directory path.
func (c *Config) ResolveLogDir() string {
	if c == nil || c.Defaults.LogDir == "" {
		home, _ := os.UserHomeDir()
		return filepath.Join(home, ".nanowave", "hook-logs")
	}
	return expandPath(c.Defaults.LogDir)
}
