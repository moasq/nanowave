package xcodegenserver

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// addPermissionInput is the input for the add_permission tool.
type addPermissionInput struct {
	Key         string `json:"key" jsonschema:"description=The Info.plist key e.g. NSCameraUsageDescription or NSLocationWhenInUseUsageDescription"`
	Description string `json:"description" jsonschema:"description=User-facing reason string shown in the permission dialog"`
	Framework   string `json:"framework" jsonschema:"description=The Apple framework that needs this permission e.g. AVFoundation or CoreLocation"`
}

type textOutput struct {
	Message string `json:"message"`
}

func handleAddPermission(ctx context.Context, req *mcp.CallToolRequest, input addPermissionInput) (*mcp.CallToolResult, textOutput, error) {
	workDir, err := os.Getwd()
	if err != nil {
		return nil, textOutput{}, fmt.Errorf("failed to get working directory: %w", err)
	}

	cfg, err := loadConfig(workDir)
	if err != nil {
		return nil, textOutput{}, err
	}

	// Check for duplicate
	for _, p := range cfg.Permissions {
		if p.Key == input.Key {
			return nil, textOutput{Message: fmt.Sprintf("Permission %s already exists", input.Key)}, nil
		}
	}

	cfg.Permissions = append(cfg.Permissions, Permission{
		Key:         input.Key,
		Description: input.Description,
		Framework:   input.Framework,
	})

	if err := applyAndRegenerate(workDir, cfg); err != nil {
		return nil, textOutput{}, err
	}

	return nil, textOutput{Message: fmt.Sprintf("Added permission %s (%s). project.yml updated and xcodegen regenerated.", input.Key, input.Framework)}, nil
}

// addExtensionInput is the input for the add_extension tool.
type addExtensionInput struct {
	Kind    string `json:"kind" jsonschema:"description=Extension type: widget live_activity share notification_service safari app_clip"`
	Name    string `json:"name" jsonschema:"description=Target name e.g. MyAppWidget. If empty a default name is generated."`
	Purpose string `json:"purpose" jsonschema:"description=What this extension does e.g. Shows daily summary on home screen"`
}

func handleAddExtension(ctx context.Context, req *mcp.CallToolRequest, input addExtensionInput) (*mcp.CallToolResult, textOutput, error) {
	workDir, err := os.Getwd()
	if err != nil {
		return nil, textOutput{}, fmt.Errorf("failed to get working directory: %w", err)
	}

	cfg, err := loadConfig(workDir)
	if err != nil {
		return nil, textOutput{}, err
	}

	// Validate extension kind against platform
	platform := cfg.Platform
	if platform == "watchos" {
		unsupported := map[string]bool{
			"live_activity": true, "share": true,
			"notification_service": true, "safari": true, "app_clip": true,
		}
		if unsupported[input.Kind] {
			return nil, textOutput{}, fmt.Errorf("extension kind %q is not supported on watchOS (only widget is supported)", input.Kind)
		}
	}

	ext := ExtensionPlan{
		Kind:    input.Kind,
		Name:    input.Name,
		Purpose: input.Purpose,
	}

	// Generate default name if not provided
	name := extensionTargetName(ext, cfg.AppName)
	ext.Name = name

	// Check for duplicate
	for _, e := range cfg.Extensions {
		if e.Name == name {
			return nil, textOutput{Message: fmt.Sprintf("Extension %s already exists", name)}, nil
		}
	}

	cfg.Extensions = append(cfg.Extensions, ext)

	// Scaffold directories
	sourcePath := filepath.Join(workDir, "Targets", name)
	sharedPath := filepath.Join(workDir, "Shared")
	for _, dir := range []string{sourcePath, sharedPath} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return nil, textOutput{}, fmt.Errorf("failed to create directory %s: %w", dir, err)
		}
	}

	// Write placeholder files so xcodegen doesn't complain
	placeholder := []byte("// Placeholder — replaced by generated code\nimport Foundation\n")
	for _, dir := range []string{sourcePath, sharedPath} {
		p := filepath.Join(dir, "Placeholder.swift")
		if _, err := os.Stat(p); os.IsNotExist(err) {
			if err := os.WriteFile(p, placeholder, 0o644); err != nil {
				return nil, textOutput{}, fmt.Errorf("failed to write placeholder: %w", err)
			}
		}
	}

	if err := applyAndRegenerate(workDir, cfg); err != nil {
		return nil, textOutput{}, err
	}

	return nil, textOutput{Message: fmt.Sprintf("Added %s extension '%s'. Created Targets/%s/ and Shared/ directories. project.yml updated and xcodegen regenerated. Extension files go in Targets/%s/, shared types (e.g. ActivityAttributes) go in Shared/.", input.Kind, name, name, name)}, nil
}

// addEntitlementInput is the input for the add_entitlement tool.
type addEntitlementInput struct {
	Target string `json:"target" jsonschema:"description=Target name to add the entitlement to. Empty or omitted means the main app target."`
	Key    string `json:"key" jsonschema:"description=Entitlement key e.g. com.apple.security.application-groups or com.apple.developer.healthkit"`
	Value  any    `json:"value" jsonschema:"description=Entitlement value - true for boolean entitlements or an array of strings for list entitlements"`
}

func handleAddEntitlement(ctx context.Context, req *mcp.CallToolRequest, input addEntitlementInput) (*mcp.CallToolResult, textOutput, error) {
	workDir, err := os.Getwd()
	if err != nil {
		return nil, textOutput{}, fmt.Errorf("failed to get working directory: %w", err)
	}

	cfg, err := loadConfig(workDir)
	if err != nil {
		return nil, textOutput{}, err
	}

	// Check for duplicate
	for i, e := range cfg.Entitlements {
		if e.Key == input.Key && e.Target == input.Target {
			cfg.Entitlements[i].Value = input.Value
			if err := applyAndRegenerate(workDir, cfg); err != nil {
				return nil, textOutput{}, err
			}
			return nil, textOutput{Message: fmt.Sprintf("Updated entitlement %s. project.yml updated and xcodegen regenerated.", input.Key)}, nil
		}
	}

	cfg.Entitlements = append(cfg.Entitlements, Entitlement{
		Key:    input.Key,
		Value:  input.Value,
		Target: input.Target,
	})

	if err := applyAndRegenerate(workDir, cfg); err != nil {
		return nil, textOutput{}, err
	}

	target := input.Target
	if target == "" {
		target = "main app"
	}
	return nil, textOutput{Message: fmt.Sprintf("Added entitlement %s to %s. project.yml updated and xcodegen regenerated.", input.Key, target)}, nil
}

// addLocalizationInput is the input for the add_localization tool.
type addLocalizationInput struct {
	Languages []string `json:"languages" jsonschema:"description=Language codes to add e.g. [en ar es]. English (en) is always included."`
}

func handleAddLocalization(ctx context.Context, req *mcp.CallToolRequest, input addLocalizationInput) (*mcp.CallToolResult, textOutput, error) {
	workDir, err := os.Getwd()
	if err != nil {
		return nil, textOutput{}, fmt.Errorf("failed to get working directory: %w", err)
	}

	cfg, err := loadConfig(workDir)
	if err != nil {
		return nil, textOutput{}, err
	}

	// Merge languages, ensuring "en" is first and no duplicates
	existing := make(map[string]bool)
	for _, l := range cfg.Localizations {
		existing[l] = true
	}
	for _, l := range input.Languages {
		if !existing[l] {
			cfg.Localizations = append(cfg.Localizations, l)
			existing[l] = true
		}
	}
	// Ensure "en" is first
	if !existing["en"] {
		cfg.Localizations = append([]string{"en"}, cfg.Localizations...)
	} else {
		// Move "en" to front if not already
		var reordered []string
		reordered = append(reordered, "en")
		for _, l := range cfg.Localizations {
			if l != "en" {
				reordered = append(reordered, l)
			}
		}
		cfg.Localizations = reordered
	}

	// Create .lproj directories
	for _, lang := range cfg.Localizations {
		lprojDir := filepath.Join(workDir, cfg.AppName, lang+".lproj")
		if err := os.MkdirAll(lprojDir, 0o755); err != nil {
			return nil, textOutput{}, fmt.Errorf("failed to create %s.lproj: %w", lang, err)
		}
	}

	if err := applyAndRegenerate(workDir, cfg); err != nil {
		return nil, textOutput{}, err
	}

	return nil, textOutput{Message: fmt.Sprintf("Localization set to %s. Created .lproj directories and updated knownRegions in project.yml. xcodegen regenerated.", strings.Join(cfg.Localizations, ", "))}, nil
}

// setBuildSettingInput is the input for the set_build_setting tool.
type setBuildSettingInput struct {
	Target string `json:"target" jsonschema:"description=Target name. Empty or omitted means the main app target."`
	Key    string `json:"key" jsonschema:"description=Build setting key e.g. TARGETED_DEVICE_FAMILY or SWIFT_STRICT_CONCURRENCY"`
	Value  string `json:"value" jsonschema:"description=Build setting value"`
}

func handleSetBuildSetting(ctx context.Context, req *mcp.CallToolRequest, input setBuildSettingInput) (*mcp.CallToolResult, textOutput, error) {
	workDir, err := os.Getwd()
	if err != nil {
		return nil, textOutput{}, fmt.Errorf("failed to get working directory: %w", err)
	}

	cfg, err := loadConfig(workDir)
	if err != nil {
		return nil, textOutput{}, err
	}

	if input.Target == "" || input.Target == cfg.AppName {
		if cfg.BuildSettings == nil {
			cfg.BuildSettings = make(map[string]string)
		}
		cfg.BuildSettings[input.Key] = input.Value
	} else {
		// Find the extension and add to its settings
		found := false
		for i, ext := range cfg.Extensions {
			if ext.Name == input.Target {
				if cfg.Extensions[i].Settings == nil {
					cfg.Extensions[i].Settings = make(map[string]string)
				}
				cfg.Extensions[i].Settings[input.Key] = input.Value
				found = true
				break
			}
		}
		if !found {
			return nil, textOutput{}, fmt.Errorf("target %s not found", input.Target)
		}
	}

	if err := applyAndRegenerate(workDir, cfg); err != nil {
		return nil, textOutput{}, err
	}

	target := input.Target
	if target == "" {
		target = "main app"
	}
	return nil, textOutput{Message: fmt.Sprintf("Set %s = %s on %s. project.yml updated and xcodegen regenerated.", input.Key, input.Value, target)}, nil
}

// getProjectConfigInput is the input for the get_project_config tool (no inputs needed).
type getProjectConfigInput struct{}

func handleGetProjectConfig(ctx context.Context, req *mcp.CallToolRequest, input getProjectConfigInput) (*mcp.CallToolResult, textOutput, error) {
	workDir, err := os.Getwd()
	if err != nil {
		return nil, textOutput{}, fmt.Errorf("failed to get working directory: %w", err)
	}

	cfg, err := loadConfig(workDir)
	if err != nil {
		return nil, textOutput{}, err
	}

	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return nil, textOutput{}, fmt.Errorf("failed to marshal config: %w", err)
	}

	var summary strings.Builder
	summary.WriteString(fmt.Sprintf("App: %s (bundle: %s)\n", cfg.AppName, cfg.BundleID))

	if cfg.Platform != "" {
		summary.WriteString(fmt.Sprintf("Platform: %s\n", cfg.Platform))
	}
	if cfg.WatchProjectShape != "" {
		summary.WriteString(fmt.Sprintf("Watch project shape: %s\n", cfg.WatchProjectShape))
	}

	if len(cfg.Permissions) > 0 {
		summary.WriteString(fmt.Sprintf("Permissions: %d\n", len(cfg.Permissions)))
		for _, p := range cfg.Permissions {
			summary.WriteString(fmt.Sprintf("  - %s (%s)\n", p.Key, p.Framework))
		}
	}

	if len(cfg.Extensions) > 0 {
		summary.WriteString(fmt.Sprintf("Extensions: %d\n", len(cfg.Extensions)))
		for _, e := range cfg.Extensions {
			summary.WriteString(fmt.Sprintf("  - %s (%s): %s\n", e.Name, e.Kind, e.Purpose))
		}
	}

	if len(cfg.Localizations) > 0 {
		summary.WriteString(fmt.Sprintf("Localizations: %s\n", strings.Join(cfg.Localizations, ", ")))
	}

	if len(cfg.Entitlements) > 0 {
		summary.WriteString(fmt.Sprintf("Entitlements: %d\n", len(cfg.Entitlements)))
		for _, e := range cfg.Entitlements {
			target := e.Target
			if target == "" {
				target = "main"
			}
			summary.WriteString(fmt.Sprintf("  - %s (target: %s)\n", e.Key, target))
		}
	}

	summary.WriteString("\nFull config:\n")
	summary.Write(data)

	return nil, textOutput{Message: summary.String()}, nil
}

// regenerateProjectInput is the input for the regenerate_project tool (no inputs needed).
type regenerateProjectInput struct{}

func handleRegenerateProject(ctx context.Context, req *mcp.CallToolRequest, input regenerateProjectInput) (*mcp.CallToolResult, textOutput, error) {
	workDir, err := os.Getwd()
	if err != nil {
		return nil, textOutput{}, fmt.Errorf("failed to get working directory: %w", err)
	}

	if err := runXcodeGen(workDir); err != nil {
		return nil, textOutput{}, err
	}

	return nil, textOutput{Message: "xcodegen generate completed successfully. .xcodeproj regenerated."}, nil
}

// applyAndRegenerate saves the config, generates project.yml, and runs xcodegen.
func applyAndRegenerate(workDir string, cfg *ProjectConfig) error {
	if err := saveConfig(workDir, cfg); err != nil {
		return fmt.Errorf("failed to save config: %w", err)
	}

	yml := generateProjectYAML(cfg)
	if err := os.WriteFile(filepath.Join(workDir, "project.yml"), []byte(yml), 0o644); err != nil {
		return fmt.Errorf("failed to write project.yml: %w", err)
	}

	if err := runXcodeGen(workDir); err != nil {
		return err
	}

	return nil
}

// runXcodeGen runs `xcodegen generate` in the given directory.
func runXcodeGen(workDir string) error {
	cmd := exec.Command("xcodegen", "generate")
	cmd.Dir = workDir
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("xcodegen generate failed: %w\n%s", err, string(output))
	}
	return nil
}
