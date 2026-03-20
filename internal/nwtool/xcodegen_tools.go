package nwtool

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// registerXcodeGenTools registers xcodegen-equivalent CLI tools for non-MCP runtimes.
func registerXcodeGenTools(r *Registry) {
	r.Register(xcodegenAddPermissionTool())
	r.Register(xcodegenAddExtensionTool())
	r.Register(xcodegenAddEntitlementTool())
	r.Register(xcodegenAddPackageTool())
	r.Register(xcodegenGetConfigTool())
	r.Register(xcodegenRegenerateTool())
}

// --- Shared config helpers ---

type projectConfig struct {
	AppName           string            `json:"app_name"`
	BundleID          string            `json:"bundle_id"`
	Platform          string            `json:"platform,omitempty"`
	WatchProjectShape string            `json:"watch_project_shape,omitempty"`
	DeviceFamily      string            `json:"device_family,omitempty"`
	Permissions       []permissionEntry `json:"permissions,omitempty"`
	Extensions        []extensionEntry  `json:"extensions,omitempty"`
	Localizations     []string          `json:"localizations,omitempty"`
	Entitlements      []entitlementEntry `json:"entitlements,omitempty"`
	BuildSettings     map[string]string `json:"build_settings,omitempty"`
	Packages          []packageDep      `json:"packages,omitempty"`
}

type permissionEntry struct {
	Key         string `json:"key"`
	Description string `json:"description"`
	Framework   string `json:"framework"`
}

type extensionEntry struct {
	Kind    string `json:"kind"`
	Name    string `json:"name"`
	Purpose string `json:"purpose"`
}

type entitlementEntry struct {
	Key    string `json:"key"`
	Value  any    `json:"value"`
	Target string `json:"target,omitempty"`
}

type packageDep struct {
	Name       string   `json:"name"`
	URL        string   `json:"url"`
	MinVersion string   `json:"min_version"`
	Products   []string `json:"products,omitempty"`
}

func loadProjectConfig(projectDir string) (*projectConfig, error) {
	data, err := os.ReadFile(filepath.Join(projectDir, "project_config.json"))
	if err != nil {
		return nil, fmt.Errorf("failed to read project_config.json: %w", err)
	}
	var cfg projectConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse project_config.json: %w", err)
	}
	return &cfg, nil
}

func saveProjectConfig(projectDir string, cfg *projectConfig) error {
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}
	return os.WriteFile(filepath.Join(projectDir, "project_config.json"), data, 0o644)
}

func regenerateXcodeProject(projectDir string) error {
	cmd := exec.Command("xcodegen", "generate")
	cmd.Dir = projectDir
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("xcodegen generate failed: %w\n%s", err, strings.TrimSpace(string(output)))
	}
	return nil
}

// --- nw_xcodegen_add_permission ---

func xcodegenAddPermissionTool() *Tool {
	return &Tool{
		Name:        "nw_xcodegen_add_permission",
		Description: "Add a permission (Info.plist key) to the Xcode project. Updates project_config.json, regenerates project.yml, and reruns xcodegen.",
		InputSchema: json.RawMessage(`{
  "type": "object",
  "properties": {
    "project_dir": {"type": "string", "description": "Absolute path to the project directory"},
    "key": {"type": "string", "description": "Info.plist key e.g. NSCameraUsageDescription"},
    "description": {"type": "string", "description": "User-facing reason string for the permission dialog"},
    "framework": {"type": "string", "description": "Apple framework requiring this permission e.g. AVFoundation"}
  },
  "required": ["project_dir", "key", "description", "framework"]
}`),
		Handler: func(_ context.Context, input json.RawMessage) (json.RawMessage, error) {
			var in struct {
				ProjectDir  string `json:"project_dir"`
				Key         string `json:"key"`
				Description string `json:"description"`
				Framework   string `json:"framework"`
			}
			if err := json.Unmarshal(input, &in); err != nil {
				return jsonError("invalid input: " + err.Error())
			}

			cfg, err := loadProjectConfig(in.ProjectDir)
			if err != nil {
				return jsonError(err.Error())
			}

			for _, p := range cfg.Permissions {
				if p.Key == in.Key {
					return jsonOK(map[string]any{"success": true, "message": fmt.Sprintf("Permission %s already exists", in.Key)})
				}
			}

			cfg.Permissions = append(cfg.Permissions, permissionEntry{Key: in.Key, Description: in.Description, Framework: in.Framework})
			if err := saveProjectConfig(in.ProjectDir, cfg); err != nil {
				return jsonError(err.Error())
			}
			if err := regenerateXcodeProject(in.ProjectDir); err != nil {
				return jsonError(err.Error())
			}
			return jsonOK(map[string]any{"success": true, "message": fmt.Sprintf("Added permission %s (%s)", in.Key, in.Framework)})
		},
	}
}

// --- nw_xcodegen_add_extension ---

func xcodegenAddExtensionTool() *Tool {
	return &Tool{
		Name:        "nw_xcodegen_add_extension",
		Description: "Add an extension target (widget, live_activity, share, notification_service, safari, app_clip) to the Xcode project. Creates the target configuration and reruns xcodegen.",
		InputSchema: json.RawMessage(`{
  "type": "object",
  "properties": {
    "project_dir": {"type": "string", "description": "Absolute path to the project directory"},
    "kind": {"type": "string", "description": "Extension type: widget, live_activity, share, notification_service, safari, app_clip"},
    "name": {"type": "string", "description": "Target name e.g. MyAppWidget"},
    "purpose": {"type": "string", "description": "What this extension does"}
  },
  "required": ["project_dir", "kind"]
}`),
		Handler: func(_ context.Context, input json.RawMessage) (json.RawMessage, error) {
			var in struct {
				ProjectDir string `json:"project_dir"`
				Kind       string `json:"kind"`
				Name       string `json:"name"`
				Purpose    string `json:"purpose"`
			}
			if err := json.Unmarshal(input, &in); err != nil {
				return jsonError("invalid input: " + err.Error())
			}

			cfg, err := loadProjectConfig(in.ProjectDir)
			if err != nil {
				return jsonError(err.Error())
			}

			cfg.Extensions = append(cfg.Extensions, extensionEntry{Kind: in.Kind, Name: in.Name, Purpose: in.Purpose})
			if err := saveProjectConfig(in.ProjectDir, cfg); err != nil {
				return jsonError(err.Error())
			}

			// Create source directory for the extension
			name := in.Name
			if name == "" {
				name = cfg.AppName + strings.ToUpper(in.Kind[:1]) + in.Kind[1:]
			}
			sourcePath := filepath.Join(in.ProjectDir, "Targets", name)
			os.MkdirAll(sourcePath, 0o755)
			os.MkdirAll(filepath.Join(in.ProjectDir, "Shared"), 0o755)

			if err := regenerateXcodeProject(in.ProjectDir); err != nil {
				return jsonError(err.Error())
			}
			return jsonOK(map[string]any{"success": true, "message": fmt.Sprintf("Added %s extension: %s", in.Kind, name)})
		},
	}
}

// --- nw_xcodegen_add_entitlement ---

func xcodegenAddEntitlementTool() *Tool {
	return &Tool{
		Name:        "nw_xcodegen_add_entitlement",
		Description: "Add an entitlement to the Xcode project (e.g. App Groups, HealthKit). Updates config and reruns xcodegen.",
		InputSchema: json.RawMessage(`{
  "type": "object",
  "properties": {
    "project_dir": {"type": "string", "description": "Absolute path to the project directory"},
    "key": {"type": "string", "description": "Entitlement key e.g. com.apple.security.application-groups"},
    "value": {"description": "Entitlement value (string, bool, or array)"},
    "target": {"type": "string", "description": "Target name (empty = main app)", "default": ""}
  },
  "required": ["project_dir", "key", "value"]
}`),
		Handler: func(_ context.Context, input json.RawMessage) (json.RawMessage, error) {
			var in struct {
				ProjectDir string `json:"project_dir"`
				Key        string `json:"key"`
				Value      any    `json:"value"`
				Target     string `json:"target"`
			}
			if err := json.Unmarshal(input, &in); err != nil {
				return jsonError("invalid input: " + err.Error())
			}

			cfg, err := loadProjectConfig(in.ProjectDir)
			if err != nil {
				return jsonError(err.Error())
			}

			cfg.Entitlements = append(cfg.Entitlements, entitlementEntry{Key: in.Key, Value: in.Value, Target: in.Target})
			if err := saveProjectConfig(in.ProjectDir, cfg); err != nil {
				return jsonError(err.Error())
			}
			if err := regenerateXcodeProject(in.ProjectDir); err != nil {
				return jsonError(err.Error())
			}
			return jsonOK(map[string]any{"success": true, "message": fmt.Sprintf("Added entitlement %s", in.Key)})
		},
	}
}

// --- nw_xcodegen_add_package ---

func xcodegenAddPackageTool() *Tool {
	return &Tool{
		Name:        "nw_xcodegen_add_package",
		Description: "Add an SPM package dependency to the Xcode project. Updates project_config.json, regenerates project.yml with the package, and reruns xcodegen.",
		InputSchema: json.RawMessage(`{
  "type": "object",
  "properties": {
    "project_dir": {"type": "string", "description": "Absolute path to the project directory"},
    "name": {"type": "string", "description": "Package repository name (last path component of URL)"},
    "url": {"type": "string", "description": "Git repository URL"},
    "min_version": {"type": "string", "description": "Minimum version e.g. 4.0.0"},
    "products": {"type": "array", "items": {"type": "string"}, "description": "Product names to link (defaults to [name])"}
  },
  "required": ["project_dir", "name", "url", "min_version"]
}`),
		Handler: func(_ context.Context, input json.RawMessage) (json.RawMessage, error) {
			var in struct {
				ProjectDir string   `json:"project_dir"`
				Name       string   `json:"name"`
				URL        string   `json:"url"`
				MinVersion string   `json:"min_version"`
				Products   []string `json:"products"`
			}
			if err := json.Unmarshal(input, &in); err != nil {
				return jsonError("invalid input: " + err.Error())
			}

			cfg, err := loadProjectConfig(in.ProjectDir)
			if err != nil {
				return jsonError(err.Error())
			}

			// Check for duplicate
			for _, p := range cfg.Packages {
				if p.Name == in.Name {
					return jsonOK(map[string]any{"success": true, "message": fmt.Sprintf("Package %s already exists", in.Name)})
				}
			}

			cfg.Packages = append(cfg.Packages, packageDep{
				Name:       in.Name,
				URL:        in.URL,
				MinVersion: in.MinVersion,
				Products:   in.Products,
			})
			if err := saveProjectConfig(in.ProjectDir, cfg); err != nil {
				return jsonError(err.Error())
			}
			if err := regenerateXcodeProject(in.ProjectDir); err != nil {
				return jsonError(err.Error())
			}
			return jsonOK(map[string]any{"success": true, "message": fmt.Sprintf("Added package %s from %s", in.Name, in.URL)})
		},
	}
}

// --- nw_xcodegen_get_config ---

func xcodegenGetConfigTool() *Tool {
	return &Tool{
		Name:        "nw_xcodegen_get_config",
		Description: "Get the current Xcode project configuration from project_config.json. Returns all targets, permissions, extensions, packages, and build settings.",
		InputSchema: json.RawMessage(`{
  "type": "object",
  "properties": {
    "project_dir": {"type": "string", "description": "Absolute path to the project directory"}
  },
  "required": ["project_dir"]
}`),
		Handler: func(_ context.Context, input json.RawMessage) (json.RawMessage, error) {
			var in struct {
				ProjectDir string `json:"project_dir"`
			}
			if err := json.Unmarshal(input, &in); err != nil {
				return jsonError("invalid input: " + err.Error())
			}

			cfg, err := loadProjectConfig(in.ProjectDir)
			if err != nil {
				return jsonError(err.Error())
			}
			return jsonOK(cfg)
		},
	}
}

// --- nw_xcodegen_regenerate ---

func xcodegenRegenerateTool() *Tool {
	return &Tool{
		Name:        "nw_xcodegen_regenerate",
		Description: "Regenerate the .xcodeproj from project.yml by running xcodegen generate. Use after manually editing project.yml.",
		InputSchema: json.RawMessage(`{
  "type": "object",
  "properties": {
    "project_dir": {"type": "string", "description": "Absolute path to the project directory"}
  },
  "required": ["project_dir"]
}`),
		Handler: func(_ context.Context, input json.RawMessage) (json.RawMessage, error) {
			var in struct {
				ProjectDir string `json:"project_dir"`
			}
			if err := json.Unmarshal(input, &in); err != nil {
				return jsonError("invalid input: " + err.Error())
			}
			if err := regenerateXcodeProject(in.ProjectDir); err != nil {
				return jsonError(err.Error())
			}
			return jsonOK(map[string]any{"success": true, "message": "xcodegen regenerated .xcodeproj"})
		},
	}
}
