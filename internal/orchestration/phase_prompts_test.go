package orchestration

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadPhaseSkillContent(t *testing.T) {
	content, err := loadPhaseSkillContent("analyzer")
	if err != nil {
		t.Fatalf("loadPhaseSkillContent(analyzer) error: %v", err)
	}
	if content == "" {
		t.Fatal("expected analyzer phase content")
	}
	if strings.HasPrefix(content, "---") {
		t.Fatal("expected frontmatter to be stripped")
	}
	if !strings.Contains(content, "# Analyzer") {
		t.Fatal("expected parent skill body content")
	}
	if !strings.Contains(content, "# Workflow") || !strings.Contains(content, "# Output Format") {
		t.Fatal("expected reference content to be included")
	}
	if strings.Index(content, "# Workflow") > strings.Index(content, "# Output Format") {
		t.Fatal("expected workflow reference before output-format reference")
	}
}

func TestLoadPhaseSkillContentMissing(t *testing.T) {
	if _, err := loadPhaseSkillContent("does-not-exist"); err == nil {
		t.Fatal("expected missing phase skill error")
	}
}

func TestComposeAnalyzerAndPlannerPromptsIncludePhaseSkillsAndHints(t *testing.T) {
	intent := &IntentDecision{
		Operation:             "build",
		PlatformHint:          PlatformWatchOS,
		WatchProjectShapeHint: WatchShapePaired,
		Confidence:            0.91,
		Reason:                "Explicit watch companion wording",
	}

	analyzer, err := composeAnalyzerSystemPrompt(intent)
	if err != nil {
		t.Fatalf("composeAnalyzerSystemPrompt() error: %v", err)
	}
	if !strings.Contains(analyzer, "Follow the attached phase skill content") {
		t.Fatal("expected minimal analyzer prompt shell")
	}
	if !strings.Contains(analyzer, "# Analyzer") {
		t.Fatal("expected analyzer phase skill content")
	}
	if !strings.Contains(analyzer, "Intent hints (advisory only") {
		t.Fatal("expected intent hints section")
	}
	if !strings.Contains(analyzer, "platform_hint: watchos") {
		t.Fatal("expected watchos hint")
	}

	if !strings.Contains(analyzer, "<constraints>") || !strings.Contains(analyzer, "</constraints>") {
		t.Fatal("expected XML constraints tags in analyzer prompt")
	}

	planner, err := composePlannerSystemPrompt(intent, PlatformWatchOS)
	if err != nil {
		t.Fatalf("composePlannerSystemPrompt() error: %v", err)
	}
	if !strings.Contains(planner, "# Planner") {
		t.Fatal("expected planner phase skill content")
	}
	if !strings.Contains(planner, "# Platform Rules") {
		t.Fatal("expected planner platform rules reference")
	}
	if !strings.Contains(planner, "<constraints>") || !strings.Contains(planner, "</constraints>") {
		t.Fatal("expected XML constraints tags in planner prompt")
	}
}

func TestComposeCoderAppendPromptIncludesPhaseSkill(t *testing.T) {
	prompt, err := composeCoderAppendPrompt("fixer", PlatformIOS)
	if err != nil {
		t.Fatalf("composeCoderAppendPrompt() error: %v", err)
	}
	if !strings.Contains(prompt, "Do not manually edit project.yml") {
		t.Fatal("expected minimal coder prompt shell")
	}
	if !strings.Contains(prompt, "# Fixer") {
		t.Fatal("expected fixer phase skill content")
	}
	if !strings.Contains(prompt, "# Error Triage") {
		t.Fatal("expected fixer reference content")
	}
	if !strings.Contains(prompt, "<constraints>") || !strings.Contains(prompt, "</constraints>") {
		t.Fatal("expected XML constraints tags in coder prompt")
	}
	if !strings.Contains(prompt, "<verification>") || !strings.Contains(prompt, "</verification>") {
		t.Fatal("expected XML verification tags in coder prompt")
	}
}

func TestFormatIntentHintsMultiPlatformInPrompt(t *testing.T) {
	intent := &IntentDecision{
		PlatformHint:  "ios",
		PlatformHints: []string{"ios", "watchos", "tvos"},
		Operation:     "build",
		Confidence:    0.9,
		Reason:        "multi-platform build",
	}
	hints := formatIntentHintsForPrompt(intent)
	if !strings.Contains(hints, "platform_hints: [ios, watchos, tvos]") {
		t.Errorf("expected platform_hints line, got:\n%s", hints)
	}
	if !strings.Contains(hints, "platform_hint: ios") {
		t.Errorf("expected platform_hint line, got:\n%s", hints)
	}
}

func TestComposeAnalyzerSystemPromptMultiPlatformHints(t *testing.T) {
	intent := &IntentDecision{
		Operation:     "build",
		PlatformHint:  "ios",
		PlatformHints: []string{"ios", "watchos", "tvos"},
		Confidence:    0.85,
		Reason:        "multi-platform request",
	}
	prompt, err := composeAnalyzerSystemPrompt(intent)
	if err != nil {
		t.Fatalf("composeAnalyzerSystemPrompt() error: %v", err)
	}
	if !strings.Contains(prompt, "platform_hints: [ios, watchos, tvos]") {
		t.Error("analyzer prompt should include multi-platform hints")
	}
}

func TestCoderPromptMacOSMentionsMacOS(t *testing.T) {
	prompt, err := composeCoderAppendPrompt("builder", PlatformMacOS)
	if err != nil {
		t.Fatalf("composeCoderAppendPrompt(builder, macOS) error: %v", err)
	}
	if !strings.Contains(prompt, "macOS") {
		t.Fatal("macOS coder prompt should mention macOS")
	}
}

func TestPlannerPromptMacOSRole(t *testing.T) {
	intent := &IntentDecision{Operation: "build", PlatformHint: PlatformMacOS}
	prompt, err := composePlannerSystemPrompt(intent, PlatformMacOS)
	if err != nil {
		t.Fatalf("composePlannerSystemPrompt(macOS) error: %v", err)
	}
	if !strings.Contains(prompt, "macOS app architect") {
		t.Fatal("macOS planner prompt should contain 'macOS app architect'")
	}
}

func TestCoderAppendPromptIncludesSelfCheck(t *testing.T) {
	prompt, err := composeCoderAppendPrompt("builder", PlatformIOS)
	if err != nil {
		t.Fatalf("composeCoderAppendPrompt(builder) error: %v", err)
	}
	if !strings.Contains(prompt, "<verification>") {
		t.Fatal("coder append prompt should include verification XML tag")
	}
	if !strings.Contains(prompt, "AppTheme.Fonts.*") {
		t.Fatal("self-check should mention AppTheme.Fonts.*")
	}
}

func TestBuildAndCompletionPromptsIncludePhaseSkillsAndRuleKeys(t *testing.T) {
	p := &Pipeline{}
	analysis := &AnalysisResult{
		AppName:     "Sample",
		Description: "Sample app",
		Features:    []Feature{{Name: "Storage", Description: "Save items"}},
		CoreFlow:    "Open -> Save",
	}
	plan := &PlannerResult{
		Platform:   PlatformIOS,
		Design:     DesignSystem{Navigation: "NavigationStack", Palette: Palette{Primary: "#111111", Secondary: "#222222", Accent: "#333333", Background: "#FFFFFF", Surface: "#F8F8F8"}, FontDesign: "default", CornerRadius: 12, Density: "standard", Surfaces: "solid", AppMood: "calm"},
		Files:      []FilePlan{{Path: "Models/Item.swift", TypeName: "Item", Purpose: "Model", Components: "struct Item", DataAccess: "in-memory", DependsOn: nil}},
		Models:     []ModelPlan{{Name: "Item", Storage: "in-memory", Properties: []PropertyPlan{{Name: "id", Type: "UUID", DefaultValue: "UUID()"}}}},
		RuleKeys:   []string{"storage"},
		BuildOrder: []string{"Models/Item.swift"},
	}

	appendPrompt, _, err := p.buildPrompts("", "Sample", t.TempDir(), analysis, plan)
	if err != nil {
		t.Fatalf("buildPrompts() error: %v", err)
	}
	if !strings.Contains(appendPrompt, "# Builder") {
		t.Fatal("expected builder phase skill content")
	}
	if !strings.Contains(appendPrompt, "# Build Loop") {
		t.Fatal("expected builder build-loop reference")
	}
	if !strings.Contains(appendPrompt, "SwiftData") {
		t.Fatal("expected existing rule_key skill injection to remain")
	}
	if !strings.Contains(appendPrompt, "<build-plan>") || !strings.Contains(appendPrompt, "</build-plan>") {
		t.Fatal("expected XML build-plan tags")
	}
	if !strings.Contains(appendPrompt, "<feature-rules>") || !strings.Contains(appendPrompt, "</feature-rules>") {
		t.Fatal("expected XML feature-rules tags")
	}

	report := &FileCompletionReport{
		TotalPlanned: 1,
		Missing: []PlannedFileStatus{{
			PlannedPath:  "Models/Item.swift",
			ResolvedPath: filepath.Join(t.TempDir(), "Sample", "Models", "Item.swift"),
			ExpectedType: "Item",
			Exists:       false,
			Valid:        false,
			Reason:       "missing file",
		}},
	}
	completionPrompt, _, err := p.completionPrompts("Sample", t.TempDir(), plan, report)
	if err != nil {
		t.Fatalf("completionPrompts() error: %v", err)
	}
	if !strings.Contains(completionPrompt, "# Completion Recovery") {
		t.Fatal("expected completion-recovery phase skill content")
	}
	if !strings.Contains(completionPrompt, "# Recovery Loop") {
		t.Fatal("expected recovery-loop reference")
	}
}

func TestComposeSelfCheckMacOS(t *testing.T) {
	check := composeSelfCheck(PlatformMacOS)
	for _, want := range []string{"CommandMenu", "FocusedValue", "keyboardShortcut"} {
		if !strings.Contains(check, want) {
			t.Fatalf("macOS self-check should contain %q", want)
		}
	}
}

func TestComposeSelfCheckIOS(t *testing.T) {
	check := composeSelfCheck(PlatformIOS)
	if strings.Contains(check, "CommandMenu") {
		t.Fatal("iOS self-check should NOT contain macOS-specific CommandMenu rule")
	}
	if !strings.Contains(check, "AppTheme.Fonts.*") {
		t.Fatal("iOS self-check should contain base AppTheme rule")
	}
}

func TestAppendXMLSection(t *testing.T) {
	var b strings.Builder
	appendXMLSection(&b, "test-tag", "hello world")
	got := b.String()
	if !strings.Contains(got, "<test-tag>") || !strings.Contains(got, "</test-tag>") {
		t.Fatalf("expected XML tags, got: %s", got)
	}
	if !strings.Contains(got, "hello world") {
		t.Fatal("expected content inside XML tags")
	}
}

func TestAppendXMLSectionEmpty(t *testing.T) {
	var b strings.Builder
	appendXMLSection(&b, "empty", "")
	if b.Len() != 0 {
		t.Fatal("expected empty output for empty content")
	}
}

func TestSharedConstraintsNoRedundantCriticalBlock(t *testing.T) {
	if strings.Contains(sharedConstraints, "CRITICAL RULES (HIGHEST PRIORITY)") {
		t.Fatal("sharedConstraints should not contain the redundant CRITICAL RULES block")
	}
}
