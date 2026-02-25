package orchestration

import (
	"strings"
	"testing"
)

func TestBuildPromptsContainsSections(t *testing.T) {
	p := &Pipeline{}

	analysis := &AnalysisResult{
		AppName:     "TestApp",
		Description: "A test application",
		Features: []Feature{
			{Name: "Weather", Description: "Shows weather data"},
		},
		CoreFlow: "Main screen shows weather",
	}

	plan := &PlannerResult{
		Platform: PlatformIOS,
		Design: DesignSystem{
			Navigation:   "tabs",
			Palette:      Palette{Primary: "#000", Secondary: "#111", Accent: "#222", Background: "#FFF", Surface: "#EEE"},
			FontDesign:   "rounded",
			CornerRadius: 12,
			Density:      "regular",
			Surfaces:     "flat",
			AppMood:      "calm",
		},
		Files: []FilePlan{
			{Path: "TestApp/ContentView.swift", TypeName: "ContentView", Purpose: "Main view", Components: "Text, VStack", DataAccess: "none"},
		},
		Models: []ModelPlan{
			{Name: "WeatherData", Storage: "memory", Properties: []PropertyPlan{{Name: "temp", Type: "Double"}}},
		},
		BuildOrder: []string{"TestApp/ContentView.swift"},
	}

	appendPrompt, userMsg, err := p.buildPrompts("", "TestApp", "", analysis, plan)
	if err != nil {
		t.Fatalf("buildPrompts() error: %v", err)
	}

	// Verify append prompt sections (XML-wrapped build plan)
	requiredSections := []string{
		"<build-plan>",
		"</build-plan>",
		"## Design",
		"## Files (build in this order)",
	}
	for _, section := range requiredSections {
		if !strings.Contains(appendPrompt, section) {
			t.Errorf("append prompt missing section %q", section)
		}
	}

	// Verify design details are present
	if !strings.Contains(appendPrompt, "tabs") {
		t.Error("append prompt missing navigation style")
	}
	if !strings.Contains(appendPrompt, "rounded") {
		t.Error("append prompt missing font design")
	}

	// Verify model details
	if !strings.Contains(appendPrompt, "WeatherData") {
		t.Error("append prompt missing model name")
	}
	if !strings.Contains(appendPrompt, "temp: Double") {
		t.Error("append prompt missing model property")
	}

	// Verify file plan entries
	if !strings.Contains(appendPrompt, "ContentView.swift") {
		t.Error("append prompt missing file path")
	}

	// Verify user message contains app info
	if !strings.Contains(userMsg, "TestApp") {
		t.Error("user message missing app name")
	}
	if !strings.Contains(userMsg, "A test application") {
		t.Error("user message missing app description")
	}
	if !strings.Contains(userMsg, "Weather") {
		t.Error("user message missing feature name")
	}
}

func TestBuildPromptsWithPermissions(t *testing.T) {
	p := &Pipeline{}

	analysis := &AnalysisResult{
		AppName:     "CamApp",
		Description: "Camera app",
		Features:    []Feature{{Name: "Camera", Description: "Take photos"}},
		CoreFlow:    "Opens camera",
	}

	plan := &PlannerResult{
		Platform: PlatformIOS,
		Design:   DesignSystem{Palette: Palette{Primary: "#000", Secondary: "#111", Accent: "#222", Background: "#FFF", Surface: "#EEE"}},
		Files:    []FilePlan{{Path: "CamApp/CameraView.swift", TypeName: "CameraView", Purpose: "Camera view"}},
		Permissions: []Permission{
			{Key: "NSCameraUsageDescription", Description: "Take photos", Framework: "AVFoundation"},
		},
		BuildOrder: []string{"CamApp/CameraView.swift"},
	}

	appendPrompt, _, err := p.buildPrompts("", "CamApp", "", analysis, plan)
	if err != nil {
		t.Fatalf("buildPrompts() error: %v", err)
	}

	if !strings.Contains(appendPrompt, "### Permissions") {
		t.Error("append prompt missing Permissions section")
	}
	if !strings.Contains(appendPrompt, "NSCameraUsageDescription") {
		t.Error("append prompt missing camera permission key")
	}
}

func TestBuildPromptsWithExtensions(t *testing.T) {
	p := &Pipeline{}

	analysis := &AnalysisResult{
		AppName:     "WidgetApp",
		Description: "Widget app",
		Features:    []Feature{{Name: "Widget", Description: "Home screen widget"}},
		CoreFlow:    "Shows widget",
	}

	plan := &PlannerResult{
		Platform: PlatformIOS,
		Design:   DesignSystem{Palette: Palette{Primary: "#000", Secondary: "#111", Accent: "#222", Background: "#FFF", Surface: "#EEE"}},
		Files:    []FilePlan{{Path: "WidgetApp/ContentView.swift", TypeName: "ContentView", Purpose: "Main view"}},
		Extensions: []ExtensionPlan{
			{Kind: "widget", Name: "StatsWidget", Purpose: "Shows stats on home screen"},
		},
		BuildOrder: []string{"WidgetApp/ContentView.swift"},
	}

	appendPrompt, _, err := p.buildPrompts("", "WidgetApp", "", analysis, plan)
	if err != nil {
		t.Fatalf("buildPrompts() error: %v", err)
	}

	if !strings.Contains(appendPrompt, "### Extensions") {
		t.Error("append prompt missing Extensions section")
	}
	if !strings.Contains(appendPrompt, "widget") {
		t.Error("append prompt missing extension kind")
	}
}

func TestBuildPromptsMultiPlatform(t *testing.T) {
	p := &Pipeline{}

	analysis := &AnalysisResult{
		AppName:     "MultiApp",
		Description: "Multi-platform app",
		Features:    []Feature{{Name: "Core", Description: "Core feature"}},
		CoreFlow:    "Main flow",
	}

	plan := &PlannerResult{
		Platform:  PlatformIOS,
		Platforms: []string{PlatformIOS, PlatformWatchOS},
		Design:    DesignSystem{Palette: Palette{Primary: "#000", Secondary: "#111", Accent: "#222", Background: "#FFF", Surface: "#EEE"}},
		Files: []FilePlan{
			{Path: "MultiApp/ContentView.swift", TypeName: "ContentView", Purpose: "iOS view", Platform: PlatformIOS},
			{Path: "MultiAppWatch/WatchView.swift", TypeName: "WatchView", Purpose: "Watch view", Platform: PlatformWatchOS},
		},
		BuildOrder: []string{"MultiApp/ContentView.swift", "MultiAppWatch/WatchView.swift"},
	}

	_, userMsg, err := p.buildPrompts("", "MultiApp", "", analysis, plan)
	if err != nil {
		t.Fatalf("buildPrompts() error: %v", err)
	}

	// Multi-platform user message should reference multiple build schemes
	if !strings.Contains(userMsg, "MULTI-PLATFORM SOURCE DIRECTORIES") {
		t.Error("user message missing multi-platform section")
	}
	if !strings.Contains(userMsg, "all builds succeed") {
		t.Error("user message missing multi-build instruction")
	}
}

func TestCompletionPromptsListsUnresolvedFiles(t *testing.T) {
	p := &Pipeline{}

	plan := &PlannerResult{
		Platform: PlatformIOS,
		Files: []FilePlan{
			{Path: "App/ContentView.swift", TypeName: "ContentView", Purpose: "Main view"},
			{Path: "App/SettingsView.swift", TypeName: "SettingsView", Purpose: "Settings"},
		},
	}

	report := &FileCompletionReport{
		TotalPlanned: 2,
		ValidCount:   1,
		Missing: []PlannedFileStatus{
			{PlannedPath: "App/SettingsView.swift", ResolvedPath: "/projects/App/App/SettingsView.swift", ExpectedType: "SettingsView", Reason: "file does not exist"},
		},
		Complete: false,
	}

	_, userMsg, err := p.completionPrompts("App", "/projects/App", plan, report)
	if err != nil {
		t.Fatalf("completionPrompts() error: %v", err)
	}

	if !strings.Contains(userMsg, "SettingsView.swift") {
		t.Error("user message missing unresolved file name")
	}
	if !strings.Contains(userMsg, "file does not exist") {
		t.Error("user message missing unresolved reason")
	}
	// ContentView should NOT be mentioned (it's valid)
	if strings.Contains(userMsg, "ContentView") {
		t.Error("user message should not mention already-valid files")
	}
}

func TestAppendBuildPlanFileEntry(t *testing.T) {
	tests := []struct {
		name string
		file FilePlan
		want string
	}{
		{
			name: "basic file",
			file: FilePlan{Path: "App/View.swift", TypeName: "MyView", Purpose: "Main view", Components: "Text", DataAccess: "none"},
			want: "- App/View.swift (MyView): Main view\n  Components: Text\n  Data access: none\n",
		},
		{
			name: "with platform tag",
			file: FilePlan{Path: "Watch/View.swift", TypeName: "WatchView", Purpose: "Watch view", Platform: PlatformWatchOS, Components: "Text", DataAccess: "none"},
			want: "- Watch/View.swift (WatchView) [watchos]: Watch view\n  Components: Text\n  Data access: none\n",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var b strings.Builder
			appendBuildPlanFileEntry(&b, tt.file)
			got := b.String()
			if got != tt.want {
				t.Errorf("appendBuildPlanFileEntry() = %q, want %q", got, tt.want)
			}
		})
	}
}
