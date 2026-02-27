package orchestration

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// writeSettingsLocal writes local (non-committed) Claude Code settings for machine-specific overrides.
func writeSettingsLocal(projectDir string) error {
	settings := `{
  "$schema": "https://json.schemastore.org/claude-code-settings.json",
  "permissions": {
    "allow": []
  }
}
`
	return os.WriteFile(filepath.Join(projectDir, ".claude", "settings.local.json"), []byte(settings), 0o644)
}

// ensureProjectConfigs writes .mcp.json, settings.json, and settings.local.json if they don't exist.
// Used by Edit and Fix flows on existing projects that may lack these files.
func ensureProjectConfigs(projectDir string) {
	mcpPath := filepath.Join(projectDir, ".mcp.json")
	if _, err := os.Stat(mcpPath); os.IsNotExist(err) {
		_ = writeMCPConfig(projectDir, nil)
	}
	claudeDir := filepath.Join(projectDir, ".claude")
	sharedSettingsPath := filepath.Join(claudeDir, "settings.json")
	if _, err := os.Stat(sharedSettingsPath); os.IsNotExist(err) {
		_ = os.MkdirAll(claudeDir, 0o755)
		_ = writeSettingsShared(projectDir, nil)
	}
	settingsPath := filepath.Join(claudeDir, "settings.local.json")
	if _, err := os.Stat(settingsPath); os.IsNotExist(err) {
		_ = os.MkdirAll(claudeDir, 0o755)
		_ = writeSettingsLocal(projectDir)
	}
}

// writeGitignore writes a standard iOS .gitignore to the project directory.
func writeGitignore(projectDir string) error {
	content := `# Xcode
*.xcodeproj/project.xcworkspace/
*.xcodeproj/xcuserdata/
xcuserdata/
DerivedData/
build/
*.ipa
*.dSYM.zip
*.dSYM

# Swift Package Manager
.build/
Package.resolved

# OS
.DS_Store

# Claude Code
.claude/settings.local.json
.claude/logs/
.claude/tmp/
.claude/transcripts/
`
	return os.WriteFile(filepath.Join(projectDir, ".gitignore"), []byte(content), 0o644)
}

// writeAssetCatalog writes the minimal Assets.xcassets structure with AppIcon and AccentColor.
func writeAssetCatalog(projectDir, appName, platform string) error {
	assetsDir := filepath.Join(projectDir, appName, "Assets.xcassets")
	appIconDir := filepath.Join(assetsDir, "AppIcon.appiconset")
	accentColorDir := filepath.Join(assetsDir, "AccentColor.colorset")

	for _, d := range []string{appIconDir, accentColorDir} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			return fmt.Errorf("failed to create asset catalog: %w", err)
		}
	}

	assetsContents := `{"info":{"version":1,"author":"xcode"}}`
	if err := os.WriteFile(filepath.Join(assetsDir, "Contents.json"), []byte(assetsContents), 0o644); err != nil {
		return err
	}

	var appIconContents string
	if IsWatchOS(platform) {
		appIconContents = `{
  "images": [
    {
      "idiom": "universal",
      "platform": "watchos",
      "size": "1024x1024"
    }
  ],
  "info": {
    "version": 1,
    "author": "xcode"
  }
}`
	} else if IsTvOS(platform) {
		// tvOS uses layered image stacks for parallax effects on the home screen.
		// The App Store icon is 1280x768 and home screen icon is 400x240.
		appIconContents = `{
  "images": [
    {
      "idiom": "tv",
      "platform": "tvos",
      "size": "1280x768",
      "scale": "1x"
    },
    {
      "idiom": "tv",
      "platform": "tvos",
      "size": "400x240",
      "scale": "2x"
    }
  ],
  "info": {
    "version": 1,
    "author": "xcode"
  }
}`
	} else if IsVisionOS(platform) {
		appIconContents = `{
  "images": [
    {
      "idiom": "universal",
      "platform": "xros",
      "size": "1024x1024"
    }
  ],
  "info": {
    "version": 1,
    "author": "xcode"
  }
}`
	} else if IsMacOS(platform) {
		appIconContents = `{
  "images": [
    {
      "idiom": "mac",
      "scale": "1x",
      "size": "1024x1024"
    }
  ],
  "info": {
    "version": 1,
    "author": "xcode"
  }
}`
	} else {
		appIconContents = `{
  "images": [
    {
      "idiom": "universal",
      "platform": "ios",
      "size": "1024x1024"
    }
  ],
  "info": {
    "version": 1,
    "author": "xcode"
  }
}`
	}
	if err := os.WriteFile(filepath.Join(appIconDir, "Contents.json"), []byte(appIconContents), 0o644); err != nil {
		return err
	}

	accentColorContents := `{
  "colors": [
    {
      "idiom": "universal"
    }
  ],
  "info": {
    "version": 1,
    "author": "xcode"
  }
}`
	return os.WriteFile(filepath.Join(accentColorDir, "Contents.json"), []byte(accentColorContents), 0o644)
}

// writeProjectConfig writes project_config.json from the PlannerResult.
// This is the source of truth that the xcodegen MCP server reads/writes.
func writeProjectConfig(projectDir string, plan *PlannerResult, appName string) error {
	bundleID := fmt.Sprintf("%s.%s", bundleIDPrefix(), strings.ToLower(appName))

	type permission struct {
		Key         string `json:"key"`
		Description string `json:"description"`
		Framework   string `json:"framework"`
	}
	type extensionPlan struct {
		Kind         string            `json:"kind"`
		Name         string            `json:"name"`
		Purpose      string            `json:"purpose"`
		InfoPlist    map[string]any    `json:"info_plist,omitempty"`
		Entitlements map[string]any    `json:"entitlements,omitempty"`
		Settings     map[string]string `json:"settings,omitempty"`
	}
	type projectConfig struct {
		AppName           string            `json:"app_name"`
		BundleID          string            `json:"bundle_id"`
		Platform          string            `json:"platform,omitempty"`
		Platforms         []string          `json:"platforms,omitempty"`
		WatchProjectShape string            `json:"watch_project_shape,omitempty"`
		DeviceFamily      string            `json:"device_family,omitempty"`
		Permissions       []permission      `json:"permissions,omitempty"`
		Extensions        []extensionPlan   `json:"extensions,omitempty"`
		Localizations     []string          `json:"localizations,omitempty"`
		BuildSettings     map[string]string `json:"build_settings,omitempty"`
	}

	cfg := projectConfig{
		AppName:           appName,
		BundleID:          bundleID,
		Platform:          plan.GetPlatform(),
		WatchProjectShape: plan.GetWatchProjectShape(),
		DeviceFamily:      plan.GetDeviceFamily(),
	}
	if plan.IsMultiPlatform() {
		cfg.Platforms = plan.GetPlatforms()
	}

	if plan != nil {
		for _, p := range plan.Permissions {
			cfg.Permissions = append(cfg.Permissions, permission{
				Key:         p.Key,
				Description: p.Description,
				Framework:   p.Framework,
			})
		}
		for _, ext := range plan.Extensions {
			name := extensionTargetName(ext, appName)
			cfg.Extensions = append(cfg.Extensions, extensionPlan{
				Kind:         ext.Kind,
				Name:         name,
				Purpose:      ext.Purpose,
				InfoPlist:    ext.InfoPlist,
				Entitlements: ext.Entitlements,
				Settings:     ext.Settings,
			})
		}
		cfg.Localizations = plan.Localizations
	}

	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal project config: %w", err)
	}
	return os.WriteFile(filepath.Join(projectDir, "project_config.json"), data, 0o644)
}

// addAutoEntitlement adds an entitlement to project_config.json without changing the plan.
// Target is the target name; empty string means the main app target.
func addAutoEntitlement(projectDir, key string, value any, target string) error {
	path := filepath.Join(projectDir, "project_config.json")
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}

	type entitlement struct {
		Key    string `json:"key"`
		Value  any    `json:"value"`
		Target string `json:"target,omitempty"`
	}

	var entitlements []entitlement
	if existing, ok := raw["entitlements"]; ok {
		_ = json.Unmarshal(existing, &entitlements)
	}

	// Check for duplicate
	for _, e := range entitlements {
		if e.Key == key && e.Target == target {
			return nil // already present
		}
	}

	entitlements = append(entitlements, entitlement{Key: key, Value: value, Target: target})
	entData, err := json.Marshal(entitlements)
	if err != nil {
		return err
	}
	raw["entitlements"] = entData

	out, err := json.MarshalIndent(raw, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, out, 0o644)
}


// readConfigEntitlements reads project_config.json and returns entitlements for the given target
// as a map suitable for writing into project.yml properties. target "" means the main app.
func readConfigEntitlements(projectDir, target string) map[string]any {
	data, err := os.ReadFile(filepath.Join(projectDir, "project_config.json"))
	if err != nil {
		return nil
	}
	var raw struct {
		Entitlements []struct {
			Key    string `json:"key"`
			Value  any    `json:"value"`
			Target string `json:"target"`
		} `json:"entitlements"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil
	}
	result := make(map[string]any)
	for _, e := range raw.Entitlements {
		if e.Target == target {
			result[e.Key] = e.Value
		}
	}
	if len(result) == 0 {
		return nil
	}
	return result
}

// writeProjectYML writes the XcodeGen project.yml, including extension targets if present.
// entitlements is a map of main-app-target entitlements to embed in project.yml properties.
func writeProjectYML(projectDir string, plan *PlannerResult, appName string, entitlements map[string]any) error {
	yml := generateProjectYAML(appName, plan, entitlements)
	return os.WriteFile(filepath.Join(projectDir, "project.yml"), []byte(yml), 0o644)
}

// scaffoldSourceDirs creates the directory structure that XcodeGen expects before generating
// the .xcodeproj. This ensures all source paths referenced in project.yml actually exist.
func scaffoldSourceDirs(projectDir, appName string, plan *PlannerResult) error {
	dirs := []string{
		filepath.Join(projectDir, appName),
	}

	if plan != nil && plan.IsMultiPlatform() {
		// Multi-platform: create source dirs for each platform
		for _, plat := range plan.GetPlatforms() {
			suffix := PlatformSourceDirSuffix(plat)
			if suffix != "" {
				dirs = append(dirs, filepath.Join(projectDir, appName+suffix))
			}
		}
		// Shared/ always created for multi-platform
		dirs = append(dirs, filepath.Join(projectDir, "Shared"))
	} else {
		// For paired watchOS apps, create the watch source directory
		if plan != nil && IsWatchOS(plan.GetPlatform()) && plan.GetWatchProjectShape() == WatchShapePaired {
			dirs = append(dirs, filepath.Join(projectDir, appName+"Watch"))
		}
		// Shared/ directory when extensions exist
		if plan != nil && len(plan.Extensions) > 0 {
			dirs = append(dirs, filepath.Join(projectDir, "Shared"))
		}
	}

	if plan != nil {
		// Targets/{ExtensionName}/ per extension
		for _, ext := range plan.Extensions {
			name := extensionTargetName(ext, appName)
			dirs = append(dirs, filepath.Join(projectDir, "Targets", name))
		}

		// .lproj directories for localizations
		for _, lang := range plan.Localizations {
			dirs = append(dirs, filepath.Join(projectDir, appName, lang+".lproj"))
		}
	}

	for _, d := range dirs {
		if err := os.MkdirAll(d, 0o755); err != nil {
			return fmt.Errorf("failed to create directory %s: %w", d, err)
		}
	}

	// Write a placeholder .swift file in each source directory so XcodeGen
	// doesn't complain about empty source groups.
	// Each placeholder uses a unique name to avoid "Multiple commands produce
	// *.stringsdata" collisions when multiple source dirs feed one target.
	type placeholderEntry struct {
		dir  string
		name string
	}
	placeholders := []placeholderEntry{
		{filepath.Join(projectDir, appName), "Placeholder.swift"},
	}

	if plan != nil && plan.IsMultiPlatform() {
		for _, plat := range plan.GetPlatforms() {
			suffix := PlatformSourceDirSuffix(plat)
			if suffix != "" {
				placeholders = append(placeholders, placeholderEntry{filepath.Join(projectDir, appName+suffix), "Placeholder.swift"})
			}
		}
		placeholders = append(placeholders, placeholderEntry{filepath.Join(projectDir, "Shared"), "SharedPlaceholder.swift"})
	} else if plan != nil && len(plan.Extensions) > 0 {
		placeholders = append(placeholders, placeholderEntry{filepath.Join(projectDir, "Shared"), "SharedPlaceholder.swift"})
	}

	if plan != nil {
		for _, ext := range plan.Extensions {
			name := extensionTargetName(ext, appName)
			placeholders = append(placeholders, placeholderEntry{filepath.Join(projectDir, "Targets", name), "Placeholder.swift"})
		}
	}

	placeholderContent := []byte("// Placeholder — replaced by generated code\nimport Foundation\n")
	for _, p := range placeholders {
		if err := os.WriteFile(filepath.Join(p.dir, p.name), placeholderContent, 0o644); err != nil {
			return fmt.Errorf("failed to write placeholder %s: %w", filepath.Join(p.dir, p.name), err)
		}
	}

	return nil
}

// cleanupScaffoldPlaceholders removes Placeholder.swift files from directories
// that now contain real generated Swift code. These scaffolding files are created
// by scaffoldSourceDirs to satisfy XcodeGen, but after the build phase writes
// real code they are no longer needed and trigger the quality-gate hook.
func cleanupScaffoldPlaceholders(projectDir, appName string, plan *PlannerResult) {
	candidates := []string{
		filepath.Join(projectDir, appName, "Placeholder.swift"),
	}
	if plan != nil && plan.IsMultiPlatform() {
		for _, plat := range plan.GetPlatforms() {
			suffix := PlatformSourceDirSuffix(plat)
			if suffix != "" {
				candidates = append(candidates, filepath.Join(projectDir, appName+suffix, "Placeholder.swift"))
			}
		}
		candidates = append(candidates, filepath.Join(projectDir, "Shared", "SharedPlaceholder.swift"))
	} else if plan != nil && len(plan.Extensions) > 0 {
		candidates = append(candidates, filepath.Join(projectDir, "Shared", "SharedPlaceholder.swift"))
	}
	if plan != nil {
		for _, ext := range plan.Extensions {
			name := extensionTargetName(ext, appName)
			candidates = append(candidates, filepath.Join(projectDir, "Targets", name, "Placeholder.swift"))
		}
	}
	for _, p := range candidates {
		if _, err := os.Stat(p); err != nil {
			continue // already gone
		}
		// Only remove if the parent directory contains at least one other .swift file
		dir := filepath.Dir(p)
		entries, err := os.ReadDir(dir)
		if err != nil {
			continue
		}
		hasOtherSwift := false
		placeholderName := filepath.Base(p)
		for _, e := range entries {
			if e.IsDir() {
				continue
			}
			if strings.HasSuffix(e.Name(), ".swift") && e.Name() != placeholderName {
				hasOtherSwift = true
				break
			}
		}
		// Also check subdirectories — the app source tree may have its Swift files in subdirs
		if !hasOtherSwift {
			_ = filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
				if err != nil || d.IsDir() {
					return nil
				}
				if strings.HasSuffix(d.Name(), ".swift") && d.Name() != placeholderName {
					hasOtherSwift = true
					return filepath.SkipAll
				}
				return nil
			})
		}
		if hasOtherSwift {
			os.Remove(p)
		}
	}
}
