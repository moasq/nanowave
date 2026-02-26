package orchestration

import (
	"strings"
	"testing"
)

func TestLookupPackageByKey(t *testing.T) {
	tests := []struct {
		key      string
		wantName string
		wantNil  bool
	}{
		// Images
		{"kingfisher", "Kingfisher", false},
		{"nuke", "Nuke", false},
		{"sdwebimage-swiftui", "SDWebImageSwiftUI", false},
		// GIF / SVG
		{"gifu", "Gifu", false},
		{"svgview", "SVGView", false},
		// Image editing
		{"brightroom", "Brightroom", false},
		{"cropviewcontroller", "CropViewController", false},
		// Audio
		{"dswaveformimage", "DSWaveformImage", false},
		{"audiokit", "AudioKit", false},
		// Animations & Effects
		{"lottie", "Lottie", false},
		{"confetti", "ConfettiSwiftUI", false},
		{"pow", "Pow", false},
		{"vortex", "Vortex", false},
		// Shimmer & Loading
		{"shimmer", "Shimmer", false},
		{"activity-indicator", "ActivityIndicatorView", false},
		// UI Components
		{"popupview", "PopupView", false},
		{"alerttoast", "AlertToast", false},
		{"whatsnewkit", "WhatsNewKit", false},
		{"concentric-onboarding", "ConcentricOnboarding", false},
		{"horizoncalendar", "HorizonCalendar", false},
		{"exyte-chat", "ExyteChat", false},
		// Text & Content
		{"markdown-ui", "MarkdownUI", false},
		{"richtextkit", "RichTextKit", false},
		// Layouts
		{"swiftui-flow", "SwiftUI-Flow", false},
		{"waterfallgrid", "WaterfallGrid", false},
		// Scanning & Codes
		{"efqrcode", "EFQRCode", false},
		// Syntax Highlighting
		{"highlightr", "Highlightr", false},
		// Keychain
		{"keychainswift", "KeychainSwift", false},
		{"valet", "Valet", false},
		// Not found
		{"nonexistent", "", true},
	}

	for _, tc := range tests {
		t.Run(tc.key, func(t *testing.T) {
			pkg := LookupPackage(tc.key)
			if tc.wantNil {
				if pkg != nil {
					t.Errorf("LookupPackage(%q) = %v, want nil", tc.key, pkg)
				}
				return
			}
			if pkg == nil {
				t.Fatalf("LookupPackage(%q) = nil, want %q", tc.key, tc.wantName)
			}
			if pkg.Name != tc.wantName {
				t.Errorf("LookupPackage(%q).Name = %q, want %q", tc.key, pkg.Name, tc.wantName)
			}
		})
	}
}

func TestLookupPackageByName(t *testing.T) {
	tests := []struct {
		name     string
		wantKey  string
		wantNil  bool
	}{
		// Case-insensitive matches
		{"Kingfisher", "kingfisher", false},
		{"kingfisher", "kingfisher", false},
		{"KINGFISHER", "kingfisher", false},
		// All packages by display name
		{"Nuke", "nuke", false},
		{"SDWebImageSwiftUI", "sdwebimage-swiftui", false},
		{"Gifu", "gifu", false},
		{"SVGView", "svgview", false},
		{"Brightroom", "brightroom", false},
		{"CropViewController", "cropviewcontroller", false},
		{"DSWaveformImage", "dswaveformimage", false},
		{"AudioKit", "audiokit", false},
		{"Lottie", "lottie", false},
		{"ConfettiSwiftUI", "confetti", false},
		{"Pow", "pow", false},
		{"Vortex", "vortex", false},
		{"Shimmer", "shimmer", false},
		{"ActivityIndicatorView", "activity-indicator", false},
		{"PopupView", "popupview", false},
		{"AlertToast", "alerttoast", false},
		{"WhatsNewKit", "whatsnewkit", false},
		{"ConcentricOnboarding", "concentric-onboarding", false},
		{"HorizonCalendar", "horizoncalendar", false},
		{"ExyteChat", "exyte-chat", false},
		{"MarkdownUI", "markdown-ui", false},
		{"RichTextKit", "richtextkit", false},
		{"SwiftUI-Flow", "swiftui-flow", false},
		{"WaterfallGrid", "waterfallgrid", false},
		{"EFQRCode", "efqrcode", false},
		{"Highlightr", "highlightr", false},
		{"KeychainSwift", "keychainswift", false},
		{"Valet", "valet", false},
		// Not found
		{"UnknownPackage", "", true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			pkg := LookupPackageByName(tc.name)
			if tc.wantNil {
				if pkg != nil {
					t.Errorf("LookupPackageByName(%q) = %v, want nil", tc.name, pkg)
				}
				return
			}
			if pkg == nil {
				t.Fatalf("LookupPackageByName(%q) = nil, want key %q", tc.name, tc.wantKey)
			}
			if pkg.Key != tc.wantKey {
				t.Errorf("LookupPackageByName(%q).Key = %q, want %q", tc.name, pkg.Key, tc.wantKey)
			}
		})
	}
}

func TestPackagesByCategory(t *testing.T) {
	images := PackagesByCategory("images")
	if len(images) < 2 {
		t.Errorf("PackagesByCategory(\"images\") returned %d packages, want at least 2", len(images))
	}

	effects := PackagesByCategory("effects")
	if len(effects) < 2 {
		t.Errorf("PackagesByCategory(\"effects\") returned %d packages, want at least 2", len(effects))
	}

	keychain := PackagesByCategory("keychain")
	if len(keychain) < 2 {
		t.Errorf("PackagesByCategory(\"keychain\") returned %d packages, want at least 2", len(keychain))
	}

	toasts := PackagesByCategory("toasts")
	if len(toasts) < 2 {
		t.Errorf("PackagesByCategory(\"toasts\") returned %d packages, want at least 2", len(toasts))
	}

	onboarding := PackagesByCategory("onboarding")
	if len(onboarding) < 2 {
		t.Errorf("PackagesByCategory(\"onboarding\") returned %d packages, want at least 2", len(onboarding))
	}

	flowLayout := PackagesByCategory("flow-layout")
	if len(flowLayout) < 1 {
		t.Errorf("PackagesByCategory(\"flow-layout\") returned %d packages, want at least 1", len(flowLayout))
	}

	waterfallGrid := PackagesByCategory("waterfall-grid")
	if len(waterfallGrid) < 1 {
		t.Errorf("PackagesByCategory(\"waterfall-grid\") returned %d packages, want at least 1", len(waterfallGrid))
	}

	syntaxHighlighting := PackagesByCategory("syntax-highlighting")
	if len(syntaxHighlighting) < 1 {
		t.Errorf("PackagesByCategory(\"syntax-highlighting\") returned %d packages, want at least 1", len(syntaxHighlighting))
	}

	empty := PackagesByCategory("nonexistent")
	if len(empty) != 0 {
		t.Errorf("PackagesByCategory(\"nonexistent\") returned %d packages, want 0", len(empty))
	}
}

func TestAllPackagesCount(t *testing.T) {
	all := AllPackages()
	if len(all) < 27 {
		t.Errorf("AllPackages() returned %d packages, want at least 27", len(all))
	}

	// Verify no duplicate keys
	keys := make(map[string]bool)
	for _, pkg := range all {
		if keys[pkg.Key] {
			t.Errorf("duplicate package key %q", pkg.Key)
		}
		keys[pkg.Key] = true
	}
}

func TestAllCategories(t *testing.T) {
	cats := AllCategories()
	if len(cats) < 15 {
		t.Errorf("AllCategories() returned %d categories, want at least 15", len(cats))
	}

	keys := make(map[string]bool)
	for _, cat := range cats {
		if cat.Key == "" {
			t.Error("category has empty key")
		}
		if cat.Label == "" {
			t.Errorf("category %q has empty label", cat.Key)
		}
		if keys[cat.Key] {
			t.Errorf("duplicate category key %q", cat.Key)
		}
		keys[cat.Key] = true
	}
}

func TestCuratedPackageFieldsNotEmpty(t *testing.T) {
	for _, pkg := range AllPackages() {
		t.Run(pkg.Key, func(t *testing.T) {
			if pkg.Key == "" {
				t.Error("empty Key")
			}
			if pkg.Name == "" {
				t.Error("empty Name")
			}
			if pkg.Category == "" {
				t.Error("empty Category")
			}
			if pkg.Description == "" {
				t.Error("empty Description")
			}
			if pkg.RepoURL == "" {
				t.Error("empty RepoURL")
			}
			if !strings.HasPrefix(pkg.RepoURL, "https://github.com/") {
				t.Errorf("RepoURL %q does not start with https://github.com/", pkg.RepoURL)
			}
			if pkg.RepoName == "" {
				t.Error("empty RepoName")
			}
			if len(pkg.Products) == 0 {
				t.Error("empty Products")
			}
			if pkg.MinVersion == "" {
				t.Error("empty MinVersion")
			}
			// Verify category exists
			found := false
			for _, cat := range AllCategories() {
				if cat.Key == pkg.Category {
					found = true
					break
				}
			}
			if !found {
				t.Errorf("package category %q not in AllCategories()", pkg.Category)
			}
		})
	}
}

func TestAppendBuildSPMSectionResolved(t *testing.T) {
	packages := []PackagePlan{
		{Name: "Kingfisher", Reason: "disk-cached image loading"},
	}
	var b strings.Builder
	appendBuildSPMSection(&b, packages, "TestApp")
	output := b.String()

	// Should contain registry details
	if !strings.Contains(output, "https://github.com/onevcat/Kingfisher") {
		t.Error("resolved package should include repo URL")
	}
	if !strings.Contains(output, "import Kingfisher") {
		t.Error("resolved package should include import statement")
	}
	if !strings.Contains(output, "from:") {
		t.Error("resolved package should include version")
	}
	// Should NOT contain "search the internet"
	if strings.Contains(output, "Search the internet") {
		t.Error("resolved package should not have search instructions")
	}
}

func TestAppendBuildSPMSectionUnresolved(t *testing.T) {
	packages := []PackagePlan{
		{Name: "SomeUnknownLib", Reason: "does something special"},
	}
	var b strings.Builder
	appendBuildSPMSection(&b, packages, "TestApp")
	output := b.String()

	if !strings.Contains(output, "SomeUnknownLib") {
		t.Error("unresolved package should include name")
	}
	if !strings.Contains(output, "WebSearch") {
		t.Error("unresolved package should have search instructions")
	}
}

func TestAppendBuildSPMSectionMixed(t *testing.T) {
	packages := []PackagePlan{
		{Name: "Lottie", Reason: "After Effects animations"},
		{Name: "MyCustomLib", Reason: "custom functionality"},
	}
	var b strings.Builder
	appendBuildSPMSection(&b, packages, "TestApp")
	output := b.String()

	// Should have both resolved and unresolved sections
	if !strings.Contains(output, "lottie-spm") {
		t.Error("resolved Lottie should include repo name lottie-spm")
	}
	if !strings.Contains(output, "MyCustomLib") {
		t.Error("unresolved package should be listed")
	}
}

func TestAppendBuildSPMSectionMultiProduct(t *testing.T) {
	packages := []PackagePlan{
		{Name: "Nuke", Reason: "image pipeline"},
	}
	var b strings.Builder
	appendBuildSPMSection(&b, packages, "TestApp")
	output := b.String()

	if !strings.Contains(output, "import Nuke") {
		t.Error("multi-product package should list Nuke import")
	}
	if !strings.Contains(output, "import NukeUI") {
		t.Error("multi-product package should list NukeUI import")
	}
	if !strings.Contains(output, "products:") {
		t.Error("multi-product package should use products: in YAML")
	}
}

func TestAppendBuildSPMSectionEmpty(t *testing.T) {
	var b strings.Builder
	appendBuildSPMSection(&b, nil, "TestApp")
	output := b.String()

	// Should still have the format reference
	if !strings.Contains(output, "XcodeGen project.yml format") {
		t.Error("empty packages should still include format reference")
	}
	// Should not have approved or unresolved sections
	if strings.Contains(output, "approved for this project") {
		t.Error("empty packages should not have approved section")
	}
}
