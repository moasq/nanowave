package nwtool

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/moasq/nanowave/internal/agentruntime"
	"github.com/moasq/nanowave/internal/orchestration"
)

// NewDefaultRegistry creates a registry with all nanowave tools registered.
func NewDefaultRegistry() *Registry {
	r := NewRegistry()
	r.Register(getSkillsTool())
	r.Register(scaffoldProjectTool())
	r.Register(setupIntegrationTool())
	r.Register(verifyFilesTool())
	r.Register(xcodeBuildTool())
	r.Register(finalizeProjectTool())
	r.Register(projectInfoTool())
	r.Register(validatePlatformTool())
	return r
}

// --- nw_get_skills ---

func getSkillsTool() *Tool {
	return &Tool{
		Name:        "nw_get_skills",
		Description: "Load embedded skill/rule content by key. Returns markdown content for the requested skills. Use this to get architecture rules, feature guides, UI patterns, and platform-specific knowledge before writing code. Common keys: swift-conventions, mvvm-architecture, file-structure, forbidden-patterns, camera, storage, authentication, supabase, revenuecat, charts, localization, dark-mode, widgets, live-activities, animations, accessibility, navigation-patterns, gestures, etc.",
		InputSchema: json.RawMessage(`{
  "type": "object",
  "properties": {
    "keys": {"type": "array", "items": {"type": "string"}, "description": "Skill/rule keys to load (e.g. [\"swift-conventions\", \"camera\", \"dark-mode\"])"},
    "list_available": {"type": "boolean", "description": "If true, returns a list of all available skill keys instead of content", "default": false}
  }
}`),
		Handler: handleGetSkills,
	}
}

func handleGetSkills(_ context.Context, input json.RawMessage) (json.RawMessage, error) {
	var in struct {
		Keys          []string `json:"keys"`
		ListAvailable bool     `json:"list_available"`
	}
	if err := json.Unmarshal(input, &in); err != nil {
		return jsonError(fmt.Sprintf("invalid input: %v", err))
	}

	if in.ListAvailable {
		keys := orchestration.ListAvailableSkillKeys()
		return jsonOK(map[string]any{"keys": keys})
	}

	if len(in.Keys) == 0 {
		return jsonError("keys array is required (or set list_available: true)")
	}

	results := make(map[string]string, len(in.Keys))
	for _, key := range in.Keys {
		content := orchestration.LoadSkillContent(key)
		if content != "" {
			results[key] = content
		}
	}

	return jsonOK(map[string]any{
		"skills":    results,
		"loaded":    len(results),
		"requested": len(in.Keys),
	})
}

// --- nw_scaffold_project ---

func scaffoldProjectTool() *Tool {
	return &Tool{
		Name:        "nw_scaffold_project",
		Description: "Scaffold the Xcode project: write project_config.json, project.yml, asset catalogs, source directories, MCP config, settings, skill files, and run xcodegen to create the .xcodeproj. This is the first tool to call for new builds — it creates the project directory and all configuration.",
		InputSchema: json.RawMessage(`{
  "type": "object",
  "properties": {
    "project_dir":   {"type": "string", "description": "Absolute path to the project directory"},
    "app_name":      {"type": "string", "description": "PascalCase app name"},
    "plan_json":     {"type": "string", "description": "JSON string of the PlannerResult"},
    "runtime_kind":  {"type": "string", "description": "Agent runtime: claude, codex, or opencode", "default": "claude"}
  },
  "required": ["project_dir", "app_name", "plan_json"]
}`),
		Handler: handleScaffoldProject,
	}
}

func handleScaffoldProject(_ context.Context, input json.RawMessage) (json.RawMessage, error) {
	var in struct {
		ProjectDir  string `json:"project_dir"`
		AppName     string `json:"app_name"`
		PlanJSON    string `json:"plan_json"`
		RuntimeKind string `json:"runtime_kind"`
	}
	if err := json.Unmarshal(input, &in); err != nil {
		return jsonError(fmt.Sprintf("invalid input: %v", err))
	}
	var plan orchestration.PlannerResult
	if err := json.Unmarshal([]byte(in.PlanJSON), &plan); err != nil {
		return jsonError(fmt.Sprintf("invalid plan_json: %v", err))
	}
	if err := orchestration.ScaffoldProjectExternal(in.ProjectDir, in.AppName, &plan); err != nil {
		return jsonError(fmt.Sprintf("scaffold failed: %v", err))
	}

	// Write MCP config and settings now that the project directory exists
	if err := orchestration.WriteMCPConfigExternal(in.ProjectDir); err != nil {
		return jsonError(fmt.Sprintf("failed to write MCP config: %v", err))
	}
	if err := orchestration.WriteSettingsSharedExternal(in.ProjectDir); err != nil {
		return jsonError(fmt.Sprintf("failed to write settings: %v", err))
	}

	// Write skill files in the native format for the active runtime
	runtimeKind := orchestration.RuntimeClaude
	if in.RuntimeKind != "" {
		runtimeKind = agentruntime.NormalizeKind(in.RuntimeKind)
		if runtimeKind == "" {
			runtimeKind = orchestration.RuntimeClaude
		}
	}
	if err := orchestration.WriteSkillsForRuntimeExternal(in.ProjectDir, plan.GetPlatform(), plan.RuleKeys, plan.Packages, runtimeKind); err != nil {
		return jsonError(fmt.Sprintf("failed to write skills: %v", err))
	}

	return jsonOK(map[string]any{
		"success":        true,
		"xcodeproj_path": filepath.Join(in.ProjectDir, in.AppName+".xcodeproj"),
	})
}

// --- nw_setup_integration ---

func setupIntegrationTool() *Tool {
	return &Tool{
		Name:        "nw_setup_integration",
		Description: "Set up a third-party integration (Supabase, RevenueCat) for the project. This triggers an interactive setup flow that configures API keys, creates backend resources, and writes MCP config. Call this BEFORE writing integration code — it ensures credentials are available. Available providers: supabase, revenuecat.",
		InputSchema: json.RawMessage(`{
  "type": "object",
  "properties": {
    "provider":    {"type": "string", "description": "Integration provider ID: 'supabase' or 'revenuecat'"},
    "app_name":    {"type": "string", "description": "PascalCase app name (used as the key for storing credentials)"},
    "project_dir": {"type": "string", "description": "Absolute path to the project directory"}
  },
  "required": ["provider", "app_name"]
}`),
		Handler: handleSetupIntegration,
	}
}

func handleSetupIntegration(ctx context.Context, input json.RawMessage) (json.RawMessage, error) {
	var in struct {
		Provider   string `json:"provider"`
		AppName    string `json:"app_name"`
		ProjectDir string `json:"project_dir"`
	}
	if err := json.Unmarshal(input, &in); err != nil {
		return jsonError(fmt.Sprintf("invalid input: %v", err))
	}
	if in.Provider == "" || in.AppName == "" {
		return jsonError("provider and app_name are required")
	}

	result, err := orchestration.SetupIntegrationExternal(ctx, in.Provider, in.AppName, in.ProjectDir)
	if err != nil {
		return jsonError(fmt.Sprintf("integration setup failed: %v", err))
	}
	return jsonOK(result)
}

// --- nw_verify_files ---

func verifyFilesTool() *Tool {
	return &Tool{
		Name:        "nw_verify_files",
		Description: "Verify that all planned files exist, are non-empty, and contain their expected types. Returns a completion report with missing/invalid file details.",
		InputSchema: json.RawMessage(`{
  "type": "object",
  "properties": {
    "project_dir": {"type": "string", "description": "Absolute path to the project directory"},
    "app_name":    {"type": "string", "description": "PascalCase app name"},
    "plan_json":   {"type": "string", "description": "JSON string of the PlannerResult"}
  },
  "required": ["project_dir", "app_name", "plan_json"]
}`),
		Handler: handleVerifyFiles,
	}
}

func handleVerifyFiles(_ context.Context, input json.RawMessage) (json.RawMessage, error) {
	var in struct {
		ProjectDir string `json:"project_dir"`
		AppName    string `json:"app_name"`
		PlanJSON   string `json:"plan_json"`
	}
	if err := json.Unmarshal(input, &in); err != nil {
		return jsonError(fmt.Sprintf("invalid input: %v", err))
	}
	var plan orchestration.PlannerResult
	if err := json.Unmarshal([]byte(in.PlanJSON), &plan); err != nil {
		return jsonError(fmt.Sprintf("invalid plan_json: %v", err))
	}
	report, err := orchestration.VerifyPlannedFilesExternal(in.ProjectDir, in.AppName, &plan)
	if err != nil {
		return jsonError(fmt.Sprintf("verification failed: %v", err))
	}
	return jsonOK(report)
}

// --- nw_xcode_build ---

func xcodeBuildTool() *Tool {
	return &Tool{
		Name:        "nw_xcode_build",
		Description: "Run xcodebuild to compile the project. Returns build output and exit code.",
		InputSchema: json.RawMessage(`{
  "type": "object",
  "properties": {
    "project_dir": {"type": "string", "description": "Absolute path to the project directory"},
    "scheme":      {"type": "string", "description": "Xcode scheme name (usually the app name)"},
    "platform":    {"type": "string", "description": "Target platform: ios, watchos, tvos, visionos, macos", "default": "ios"},
    "destination": {"type": "string", "description": "Build destination. Auto-detected from platform if omitted."},
    "simulator":   {"type": "boolean", "description": "If true, build for simulator instead of device", "default": false}
  },
  "required": ["project_dir", "scheme"]
}`),
		Handler: handleXcodeBuild,
	}
}

func handleXcodeBuild(ctx context.Context, input json.RawMessage) (json.RawMessage, error) {
	var in struct {
		ProjectDir  string `json:"project_dir"`
		Scheme      string `json:"scheme"`
		Platform    string `json:"platform"`
		Destination string `json:"destination"`
		Simulator   bool   `json:"simulator"`
	}
	if err := json.Unmarshal(input, &in); err != nil {
		return jsonError(fmt.Sprintf("invalid input: %v", err))
	}
	if in.ProjectDir == "" || in.Scheme == "" {
		return jsonError("project_dir and scheme are required")
	}
	if in.Platform == "" {
		in.Platform = "ios"
	}

	destination := in.Destination
	if destination == "" {
		if in.Simulator {
			destination = orchestration.PlatformSimulatorDestination(in.Platform)
		} else {
			destination = orchestration.PlatformBuildDestination(in.Platform)
		}
	}

	entries, err := os.ReadDir(in.ProjectDir)
	if err != nil {
		return jsonError(fmt.Sprintf("failed to read project dir: %v", err))
	}
	var xcodeprojName string
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".xcodeproj") {
			xcodeprojName = e.Name()
			break
		}
	}
	if xcodeprojName == "" {
		return jsonError("no .xcodeproj found in project directory")
	}

	args := []string{
		"-project", xcodeprojName,
		"-scheme", in.Scheme,
		"-destination", destination,
		"-quiet", "build",
	}
	if !in.Simulator && in.Platform != "macos" {
		args = append(args, "CODE_SIGNING_ALLOWED=NO")
	}

	cmd := exec.CommandContext(ctx, "xcodebuild", args...)
	cmd.Dir = in.ProjectDir
	output, cmdErr := cmd.CombinedOutput()

	exitCode := 0
	success := true
	if cmdErr != nil {
		success = false
		if exitErr, ok := cmdErr.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else {
			exitCode = 1
		}
	}

	outputStr := string(output)
	if len(outputStr) > 8000 {
		outputStr = outputStr[len(outputStr)-8000:]
	}

	return jsonOK(map[string]any{
		"success":   success,
		"output":    outputStr,
		"exit_code": exitCode,
	})
}

// --- nw_finalize_project ---

func finalizeProjectTool() *Tool {
	return &Tool{
		Name:        "nw_finalize_project",
		Description: "Finalize a newly built project: ensure .xcodeproj exists, then git init and commit all files.",
		InputSchema: json.RawMessage(`{
  "type": "object",
  "properties": {
    "project_dir": {"type": "string", "description": "Absolute path to the project directory"},
    "app_name":    {"type": "string", "description": "PascalCase app name"}
  },
  "required": ["project_dir", "app_name"]
}`),
		Handler: handleFinalizeProject,
	}
}

func handleFinalizeProject(ctx context.Context, input json.RawMessage) (json.RawMessage, error) {
	var in struct {
		ProjectDir string `json:"project_dir"`
		AppName    string `json:"app_name"`
	}
	if err := json.Unmarshal(input, &in); err != nil {
		return jsonError(fmt.Sprintf("invalid input: %v", err))
	}

	xcodeprojPath := filepath.Join(in.ProjectDir, in.AppName+".xcodeproj")
	if _, err := os.Stat(xcodeprojPath); os.IsNotExist(err) {
		orchestration.RunXcodeGenExternal(in.ProjectDir)
	}

	var commitSHA string
	for _, step := range []struct {
		name string
		args []string
	}{
		{"git init", []string{"init"}},
		{"git add", []string{"add", "-A"}},
		{"git commit", []string{"commit", "-m", fmt.Sprintf("Initial build: %s", in.AppName)}},
	} {
		cmd := exec.CommandContext(ctx, "git", step.args...)
		cmd.Dir = in.ProjectDir
		output, err := cmd.CombinedOutput()
		if err != nil && step.name == "git commit" {
			return jsonError(fmt.Sprintf("%s failed: %v\n%s", step.name, err, string(output)))
		}
		if step.name == "git commit" {
			revCmd := exec.CommandContext(ctx, "git", "rev-parse", "HEAD")
			revCmd.Dir = in.ProjectDir
			if sha, err := revCmd.Output(); err == nil {
				commitSHA = strings.TrimSpace(string(sha))
			}
		}
	}

	return jsonOK(map[string]any{"success": true, "commit_sha": commitSHA})
}

// --- nw_project_info ---

func projectInfoTool() *Tool {
	return &Tool{
		Name:        "nw_project_info",
		Description: "Read project metadata from project_config.json.",
		InputSchema: json.RawMessage(`{
  "type": "object",
  "properties": {
    "project_dir": {"type": "string", "description": "Absolute path to the project directory"}
  },
  "required": ["project_dir"]
}`),
		Handler: handleProjectInfo,
	}
}

func handleProjectInfo(_ context.Context, input json.RawMessage) (json.RawMessage, error) {
	var in struct {
		ProjectDir string `json:"project_dir"`
	}
	if err := json.Unmarshal(input, &in); err != nil {
		return jsonError(fmt.Sprintf("invalid input: %v", err))
	}
	data, err := os.ReadFile(filepath.Join(in.ProjectDir, "project_config.json"))
	if err != nil {
		return jsonError(fmt.Sprintf("failed to read project_config.json: %v", err))
	}
	return data, nil
}

// --- nw_validate_platform ---

func validatePlatformTool() *Tool {
	return &Tool{
		Name:        "nw_validate_platform",
		Description: "Validate platform compatibility for features and extensions.",
		InputSchema: json.RawMessage(`{
  "type": "object",
  "properties": {
    "platform":   {"type": "string", "description": "Target platform: ios, watchos, tvos, visionos, macos"},
    "features":   {"type": "array", "items": {"type": "string"}, "description": "Feature rule keys to validate"},
    "extensions": {"type": "array", "items": {"type": "object", "properties": {"kind": {"type": "string"}, "name": {"type": "string"}}}, "description": "Extension plans to validate"}
  },
  "required": ["platform"]
}`),
		Handler: handleValidatePlatform,
	}
}

func handleValidatePlatform(_ context.Context, input json.RawMessage) (json.RawMessage, error) {
	var in struct {
		Platform   string                        `json:"platform"`
		Features   []string                      `json:"features"`
		Extensions []orchestration.ExtensionPlan `json:"extensions"`
	}
	if err := json.Unmarshal(input, &in); err != nil {
		return jsonError(fmt.Sprintf("invalid input: %v", err))
	}
	if err := orchestration.ValidatePlatform(in.Platform); err != nil {
		return jsonError(fmt.Sprintf("invalid platform: %v", err))
	}

	var errors []string
	if len(in.Extensions) > 0 {
		if err := orchestration.ValidateExtensionsForPlatform(in.Platform, in.Extensions); err != nil {
			errors = append(errors, err.Error())
		}
	}
	var filtered, removed []string
	if len(in.Features) > 0 {
		filtered, removed = orchestration.FilterRuleKeysForPlatform(in.Platform, in.Features)
	}

	return jsonOK(map[string]any{
		"valid":    len(errors) == 0,
		"errors":   errors,
		"filtered": filtered,
		"removed":  removed,
	})
}
