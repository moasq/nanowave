package orchestration

import (
	"os"
	"path/filepath"
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

	appendPrompt, userMsg, err := p.buildPrompts("", "TestApp", "", analysis, plan, false)
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

	appendPrompt, _, err := p.buildPrompts("", "CamApp", "", analysis, plan, false)
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

	appendPrompt, _, err := p.buildPrompts("", "WidgetApp", "", analysis, plan, false)
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

	_, userMsg, err := p.buildPrompts("", "MultiApp", "", analysis, plan, false)
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

func TestBuildPromptsAppearanceDark(t *testing.T) {
	p := &Pipeline{}
	analysis := &AnalysisResult{
		AppName:     "DarkApp",
		Description: "A dark-themed music player",
		Features:    []Feature{{Name: "Player", Description: "Music playback"}},
		CoreFlow:    "Play music",
	}
	plan := &PlannerResult{
		Platform: PlatformIOS,
		Design: DesignSystem{
			Palette: Palette{Primary: "#BB86FC", Secondary: "#03DAC6", Accent: "#CF6679", Background: "#1A1A2E", Surface: "#2D2D44"},
		},
		Files:      []FilePlan{{Path: "DarkApp/PlayerView.swift", TypeName: "PlayerView", Purpose: "Player view"}},
		BuildOrder: []string{"DarkApp/PlayerView.swift"},
	}

	appendPrompt, _, err := p.buildPrompts("", "DarkApp", "", analysis, plan, false)
	if err != nil {
		t.Fatalf("buildPrompts() error: %v", err)
	}

	if !strings.Contains(appendPrompt, "locked to Dark") {
		t.Error("build prompt for dark palette should contain 'locked to Dark'")
	}
	if !strings.Contains(appendPrompt, "Color(.label)") {
		t.Error("build prompt for dark palette should mention Color(.label) adaptive text")
	}
}

func TestBuildPromptsAppearanceLight(t *testing.T) {
	p := &Pipeline{}
	analysis := &AnalysisResult{
		AppName:     "LightApp",
		Description: "A light-themed notes app",
		Features:    []Feature{{Name: "Notes", Description: "Note taking"}},
		CoreFlow:    "Write notes",
	}
	plan := &PlannerResult{
		Platform: PlatformIOS,
		Design: DesignSystem{
			Palette: Palette{Primary: "#007AFF", Secondary: "#5856D6", Accent: "#FF9500", Background: "#F5F5F5", Surface: "#FFFFFF"},
		},
		Files:      []FilePlan{{Path: "LightApp/NotesView.swift", TypeName: "NotesView", Purpose: "Notes view"}},
		BuildOrder: []string{"LightApp/NotesView.swift"},
	}

	appendPrompt, _, err := p.buildPrompts("", "LightApp", "", analysis, plan, false)
	if err != nil {
		t.Fatalf("buildPrompts() error: %v", err)
	}

	if !strings.Contains(appendPrompt, "locked to Light") {
		t.Error("build prompt for light palette should contain 'locked to Light'")
	}
	if !strings.Contains(appendPrompt, "Color(.label)") {
		t.Error("build prompt for light palette should mention Color(.label) adaptive text")
	}
}

func TestBuildPromptsAppearanceAdaptive(t *testing.T) {
	p := &Pipeline{}
	analysis := &AnalysisResult{
		AppName:     "AdaptApp",
		Description: "An adaptive app",
		Features:    []Feature{{Name: "Core", Description: "Core feature"}},
		CoreFlow:    "Main flow",
	}
	plan := &PlannerResult{
		Platform: PlatformIOS,
		Design: DesignSystem{
			Palette: Palette{Primary: "#000", Secondary: "#111", Accent: "#222", Background: "#1A1A2E", Surface: "#333"},
		},
		RuleKeys:   []string{"dark-mode"},
		Files:      []FilePlan{{Path: "AdaptApp/ContentView.swift", TypeName: "ContentView", Purpose: "Main view"}},
		BuildOrder: []string{"AdaptApp/ContentView.swift"},
	}

	appendPrompt, _, err := p.buildPrompts("", "AdaptApp", "", analysis, plan, false)
	if err != nil {
		t.Fatalf("buildPrompts() error: %v", err)
	}

	if !strings.Contains(appendPrompt, "adaptive") {
		t.Error("build prompt with dark-mode rule should contain 'adaptive'")
	}
	if !strings.Contains(appendPrompt, "Color(.label)") {
		t.Error("build prompt with dark-mode rule should mention Color(.label) adaptive text")
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

// --- App Scenario SPM Integration Tests ---
// Each test simulates a realistic app where SPM packages are genuinely needed,
// verifying the full pipeline: PlannerResult → buildPrompts() → correct SPM output.

func TestSPMScenarioPhotoGalleryApp(t *testing.T) {
	// Photo gallery app: remote images with disk caching + Pinterest masonry grid.
	// Kingfisher: AsyncImage has no disk cache, prefetch, or downsampling.
	// WaterfallGrid: LazyVGrid forces equal row heights — can't do masonry.
	p := &Pipeline{}
	analysis := &AnalysisResult{
		AppName:     "Pictura",
		Description: "A photo gallery app with masonry grid layout",
		Features: []Feature{
			{Name: "Gallery", Description: "Pinterest-style photo grid with remote images"},
			{Name: "Detail", Description: "Full-screen photo viewer"},
		},
		CoreFlow: "Browse photo grid, tap for detail",
	}
	plan := &PlannerResult{
		Platform: PlatformIOS,
		Design:   DesignSystem{Palette: Palette{Primary: "#2D6A4F", Secondary: "#40916C", Accent: "#52B788", Background: "#F0F4F0", Surface: "#FFFFFF"}},
		Files: []FilePlan{
			{Path: "Pictura/Features/Gallery/GalleryView.swift", TypeName: "GalleryView", Purpose: "Masonry photo grid"},
			{Path: "Pictura/Features/Detail/PhotoDetailView.swift", TypeName: "PhotoDetailView", Purpose: "Full-screen viewer"},
		},
		Packages: []PackagePlan{
			{Name: "Kingfisher", Reason: "Disk-cached image loading with prefetch and downsampling for photo grid — AsyncImage has no disk cache"},
			{Name: "WaterfallGrid", Reason: "Pinterest-style staggered grid layout — native LazyVGrid forces equal row heights"},
		},
		BuildOrder: []string{"Pictura/Features/Gallery/GalleryView.swift", "Pictura/Features/Detail/PhotoDetailView.swift"},
	}

	appendPrompt, _, err := p.buildPrompts("", "Pictura", "", analysis, plan, false)
	if err != nil {
		t.Fatalf("buildPrompts() error: %v", err)
	}

	// Verify Kingfisher registry details
	assertContains(t, appendPrompt, "https://github.com/onevcat/Kingfisher", "Kingfisher repo URL")
	assertContains(t, appendPrompt, "import Kingfisher", "Kingfisher import")
	assertContains(t, appendPrompt, `from: "8.1.0"`, "Kingfisher version")

	// Verify WaterfallGrid registry details
	assertContains(t, appendPrompt, "https://github.com/paololeonardi/WaterfallGrid", "WaterfallGrid repo URL")
	assertContains(t, appendPrompt, "import WaterfallGrid", "WaterfallGrid import")

	// Both resolved — no internet search instructions
	assertNotContains(t, appendPrompt, "WebSearch", "no WebSearch for resolved packages")
}

func TestSPMScenarioCodeNotesApp(t *testing.T) {
	// Developer notes app: renders Markdown content with embedded code blocks.
	// MarkdownUI: SwiftUI has no Markdown rendering view (Text supports only inline Markdown).
	// Highlightr: zero native code syntax highlighting API exists.
	p := &Pipeline{}
	analysis := &AnalysisResult{
		AppName:     "DevNotes",
		Description: "A developer notes app with Markdown and code highlighting",
		Features: []Feature{
			{Name: "NoteList", Description: "List of notes"},
			{Name: "NoteEditor", Description: "Markdown editor with live preview"},
		},
		CoreFlow: "Browse notes, edit with Markdown preview",
	}
	plan := &PlannerResult{
		Platform: PlatformIOS,
		Design:   DesignSystem{Palette: Palette{Primary: "#5B21B6", Secondary: "#7C3AED", Accent: "#A78BFA", Background: "#1E1E2E", Surface: "#2D2D44"}},
		Files: []FilePlan{
			{Path: "DevNotes/Features/NoteList/NoteListView.swift", TypeName: "NoteListView", Purpose: "Note listing"},
			{Path: "DevNotes/Features/Editor/NoteEditorView.swift", TypeName: "NoteEditorView", Purpose: "Markdown editor"},
		},
		Packages: []PackagePlan{
			{Name: "MarkdownUI", Reason: "Render GitHub Flavored Markdown as native SwiftUI views — Text() only supports inline Markdown"},
			{Name: "Highlightr", Reason: "Syntax highlighting for 185+ languages in code blocks — no native API exists"},
		},
		BuildOrder: []string{"DevNotes/Features/NoteList/NoteListView.swift", "DevNotes/Features/Editor/NoteEditorView.swift"},
	}

	appendPrompt, _, err := p.buildPrompts("", "DevNotes", "", analysis, plan, false)
	if err != nil {
		t.Fatalf("buildPrompts() error: %v", err)
	}

	// Verify MarkdownUI registry details
	assertContains(t, appendPrompt, "https://github.com/gonzalezreal/swift-markdown-ui", "MarkdownUI repo URL")
	assertContains(t, appendPrompt, "swift-markdown-ui", "MarkdownUI XcodeGen key")
	assertContains(t, appendPrompt, "import MarkdownUI", "MarkdownUI import")

	// Verify Highlightr registry details
	assertContains(t, appendPrompt, "https://github.com/raspu/Highlightr", "Highlightr repo URL")
	assertContains(t, appendPrompt, "import Highlightr", "Highlightr import")

	// Both resolved
	assertNotContains(t, appendPrompt, "WebSearch", "no WebSearch for resolved packages")
}

func TestSPMScenarioMusicVisualizerApp(t *testing.T) {
	// Music visualizer: audio processing + waveform display + Lottie animations.
	// AudioKit: building audio synthesis on AVAudioEngine is 500+ lines of DSP code.
	// DSWaveformImage: two products (DSWaveformImage + DSWaveformImageViews).
	// Lottie: After Effects JSON playback has no native equivalent.
	p := &Pipeline{}
	analysis := &AnalysisResult{
		AppName:     "SoundWave",
		Description: "A music visualizer with waveforms and animations",
		Features: []Feature{
			{Name: "Player", Description: "Audio player with waveform visualization"},
			{Name: "Visualizer", Description: "Animated visualizations synced to audio"},
		},
		CoreFlow: "Play audio, see waveform and animations",
	}
	plan := &PlannerResult{
		Platform: PlatformIOS,
		Design:   DesignSystem{Palette: Palette{Primary: "#EF4444", Secondary: "#F97316", Accent: "#FBBF24", Background: "#0F0F23", Surface: "#1A1A2E"}},
		Files: []FilePlan{
			{Path: "SoundWave/Features/Player/PlayerView.swift", TypeName: "PlayerView", Purpose: "Audio player"},
			{Path: "SoundWave/Features/Visualizer/VisualizerView.swift", TypeName: "VisualizerView", Purpose: "Audio visualization"},
		},
		Packages: []PackagePlan{
			{Name: "AudioKit", Reason: "Audio synthesis and analysis — building this on AVAudioEngine requires 500+ lines of DSP code"},
			{Name: "DSWaveformImage", Reason: "Audio waveform visualization with native SwiftUI views"},
			{Name: "Lottie", Reason: "After Effects vector animation playback — no native equivalent"},
		},
		BuildOrder: []string{"SoundWave/Features/Player/PlayerView.swift", "SoundWave/Features/Visualizer/VisualizerView.swift"},
	}

	appendPrompt, _, err := p.buildPrompts("", "SoundWave", "", analysis, plan, false)
	if err != nil {
		t.Fatalf("buildPrompts() error: %v", err)
	}

	// Verify AudioKit — single product
	assertContains(t, appendPrompt, "https://github.com/AudioKit/AudioKit", "AudioKit repo URL")
	assertContains(t, appendPrompt, "import AudioKit", "AudioKit import")

	// Verify DSWaveformImage — multi-product package
	assertContains(t, appendPrompt, "https://github.com/dmrschmidt/DSWaveformImage", "DSWaveformImage repo URL")
	assertContains(t, appendPrompt, "import DSWaveformImage", "DSWaveformImage import")
	assertContains(t, appendPrompt, "import DSWaveformImageViews", "DSWaveformImageViews import")
	assertContains(t, appendPrompt, "products:", "multi-product YAML format")

	// Verify Lottie uses lottie-spm repo name
	assertContains(t, appendPrompt, "lottie-spm", "Lottie XcodeGen key")
	assertContains(t, appendPrompt, "import Lottie", "Lottie import")

	// All 3 resolved
	assertNotContains(t, appendPrompt, "WebSearch", "no WebSearch for resolved packages")
}

func TestSPMScenarioSocialChatApp(t *testing.T) {
	// Social chat app: pre-built chat UI + image loading + toast notifications.
	// ExyteChat: building a full chat UI with media, replies, and links is 1000+ lines.
	// Nuke: multi-product (Nuke + NukeUI) for chat message images.
	// PopupView: SwiftUI has no native toast/HUD notification API.
	p := &Pipeline{}
	analysis := &AnalysisResult{
		AppName:     "Chattr",
		Description: "A social messaging app",
		Features: []Feature{
			{Name: "Chat", Description: "Real-time messaging with media"},
			{Name: "Contacts", Description: "User contact list"},
		},
		CoreFlow: "Select contact, send messages with images",
	}
	plan := &PlannerResult{
		Platform: PlatformIOS,
		Design:   DesignSystem{Palette: Palette{Primary: "#FF6B6B", Secondary: "#EE5A5A", Accent: "#FF8787", Background: "#FFFFFF", Surface: "#F8F9FA"}},
		Files: []FilePlan{
			{Path: "Chattr/Features/Chat/ChatView.swift", TypeName: "ChatView", Purpose: "Chat messages"},
			{Path: "Chattr/Features/Contacts/ContactListView.swift", TypeName: "ContactListView", Purpose: "Contact list"},
		},
		Packages: []PackagePlan{
			{Name: "ExyteChat", Reason: "Pre-built chat message UI with media, replies, and link previews — building from scratch is 1000+ lines"},
			{Name: "Nuke", Reason: "High-performance image pipeline with disk caching for chat message images"},
			{Name: "PopupView", Reason: "Toast notifications for message sent/failed — SwiftUI has no native toast API"},
		},
		BuildOrder: []string{"Chattr/Features/Contacts/ContactListView.swift", "Chattr/Features/Chat/ChatView.swift"},
	}

	appendPrompt, _, err := p.buildPrompts("", "Chattr", "", analysis, plan, false)
	if err != nil {
		t.Fatalf("buildPrompts() error: %v", err)
	}

	// Verify ExyteChat — product name differs from repo name
	assertContains(t, appendPrompt, "https://github.com/exyte/Chat", "ExyteChat repo URL")
	assertContains(t, appendPrompt, "Chat:", "ExyteChat XcodeGen package key")
	assertContains(t, appendPrompt, "import ExyteChat", "ExyteChat import")
	assertContains(t, appendPrompt, "product: ExyteChat", "product differs from package key")

	// Verify Nuke multi-product
	assertContains(t, appendPrompt, "import Nuke", "Nuke import")
	assertContains(t, appendPrompt, "import NukeUI", "NukeUI import")

	// Verify PopupView
	assertContains(t, appendPrompt, "https://github.com/exyte/PopupView", "PopupView repo URL")

	// All resolved
	assertNotContains(t, appendPrompt, "WebSearch", "no WebSearch for resolved packages")
}

func TestSPMScenarioRecipeTagsApp(t *testing.T) {
	// Recipe app with ingredient tags: flow/wrap layout + shimmer loading.
	// SwiftUI-Flow: native HStack/VStack don't wrap to next line.
	// Shimmer: animated shimmer loading while recipes load.
	// Also includes an unknown package to test the unresolved fallback path.
	p := &Pipeline{}
	analysis := &AnalysisResult{
		AppName:     "Yummly",
		Description: "A recipe app with ingredient tagging and search",
		Features: []Feature{
			{Name: "RecipeList", Description: "Browse recipes with shimmer loading"},
			{Name: "RecipeDetail", Description: "Recipe detail with ingredient tags"},
		},
		CoreFlow: "Browse recipes, view details with ingredient tags",
	}
	plan := &PlannerResult{
		Platform: PlatformIOS,
		Design:   DesignSystem{Palette: Palette{Primary: "#E07A5F", Secondary: "#F2CC8F", Accent: "#81B29A", Background: "#FFF8F0", Surface: "#FFFFFF"}},
		Files: []FilePlan{
			{Path: "Yummly/Features/RecipeList/RecipeListView.swift", TypeName: "RecipeListView", Purpose: "Recipe browsing"},
			{Path: "Yummly/Features/RecipeDetail/RecipeDetailView.swift", TypeName: "RecipeDetailView", Purpose: "Recipe detail"},
		},
		Packages: []PackagePlan{
			{Name: "SwiftUI-Flow", Reason: "Wrapping layout for ingredient tags — native HStack does not wrap to next line"},
			{Name: "Shimmer", Reason: "Animated shimmer loading effect while recipes load from network"},
			{Name: "RecipeMagicLib", Reason: "Hypothetical recipe parsing library"},
		},
		BuildOrder: []string{"Yummly/Features/RecipeList/RecipeListView.swift", "Yummly/Features/RecipeDetail/RecipeDetailView.swift"},
	}

	appendPrompt, _, err := p.buildPrompts("", "Yummly", "", analysis, plan, false)
	if err != nil {
		t.Fatalf("buildPrompts() error: %v", err)
	}

	// Verify SwiftUI-Flow resolved
	assertContains(t, appendPrompt, "https://github.com/tevelee/SwiftUI-Flow", "SwiftUI-Flow repo URL")
	assertContains(t, appendPrompt, "import Flow", "Flow import (product name)")
	assertContains(t, appendPrompt, "SwiftUI-Flow:", "SwiftUI-Flow XcodeGen key")
	assertContains(t, appendPrompt, "product: Flow", "product differs from package key")

	// Verify Shimmer resolved
	assertContains(t, appendPrompt, "https://github.com/markiv/SwiftUI-Shimmer", "Shimmer repo URL")
	assertContains(t, appendPrompt, "import Shimmer", "Shimmer import")

	// Verify unresolved package triggers WebSearch fallback
	assertContains(t, appendPrompt, "RecipeMagicLib", "unresolved package name present")
	assertContains(t, appendPrompt, "WebSearch", "unresolved package triggers search instructions")

	// Verify both sections coexist
	assertContains(t, appendPrompt, "approved for this project", "resolved section header")
	assertContains(t, appendPrompt, "not in the curated registry", "unresolved section header")
}

// --- SPM Core Rules Enrichment Tests ---

func TestWriteCoreRulesPhotoGalleryEnrichment(t *testing.T) {
	projectDir := t.TempDir()
	rulesDir := filepath.Join(projectDir, ".claude", "rules")
	if err := os.MkdirAll(rulesDir, 0o755); err != nil {
		t.Fatalf("failed to create rules dir: %v", err)
	}

	packages := []PackagePlan{
		{Name: "Kingfisher", Reason: "Disk-cached image loading with prefetch"},
		{Name: "WaterfallGrid", Reason: "Pinterest-style staggered grid layout"},
	}

	if err := writeCoreRules(projectDir, PlatformIOS, packages); err != nil {
		t.Fatalf("writeCoreRules() error: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(rulesDir, "forbidden-patterns.md"))
	if err != nil {
		t.Fatalf("failed to read forbidden-patterns.md: %v", err)
	}
	text := string(data)

	assertContains(t, text, "Approved Packages for This Project", "approved section header")
	assertContains(t, text, "Kingfisher", "Kingfisher listed")
	assertContains(t, text, "https://github.com/onevcat/Kingfisher", "Kingfisher URL enriched")
	assertContains(t, text, "WaterfallGrid", "WaterfallGrid listed")
	assertContains(t, text, "https://github.com/paololeonardi/WaterfallGrid", "WaterfallGrid URL enriched")
	assertNotContains(t, text, "APPROVED_PACKAGES_PLACEHOLDER", "placeholder replaced")
}

func TestWriteCoreRulesMixedResolvedAndUnresolved(t *testing.T) {
	projectDir := t.TempDir()
	rulesDir := filepath.Join(projectDir, ".claude", "rules")
	if err := os.MkdirAll(rulesDir, 0o755); err != nil {
		t.Fatalf("failed to create rules dir: %v", err)
	}

	packages := []PackagePlan{
		{Name: "Lottie", Reason: "After Effects animation playback"},
		{Name: "UnknownLib", Reason: "Some custom functionality"},
	}

	if err := writeCoreRules(projectDir, PlatformIOS, packages); err != nil {
		t.Fatalf("writeCoreRules() error: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(rulesDir, "forbidden-patterns.md"))
	if err != nil {
		t.Fatalf("failed to read forbidden-patterns.md: %v", err)
	}
	text := string(data)

	// Lottie should be enriched with registry details
	assertContains(t, text, "lottie-spm", "Lottie XcodeGen key enriched")
	// UnknownLib should still appear (just without enrichment)
	assertContains(t, text, "UnknownLib", "unresolved package still listed")
}

// --- Test Helpers ---

func assertContains(t *testing.T, text, substr, label string) {
	t.Helper()
	if !strings.Contains(text, substr) {
		t.Errorf("%s: expected %q in output", label, substr)
	}
}

func assertNotContains(t *testing.T, text, substr, label string) {
	t.Helper()
	if strings.Contains(text, substr) {
		t.Errorf("%s: unexpected %q in output", label, substr)
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
