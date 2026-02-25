package xcodegenserver

import (
	"strings"
	"testing"
)

func TestGenerateProjectYAMLPairedWatchIncludesCompanionBundleIdentifier(t *testing.T) {
	cfg := &ProjectConfig{
		AppName:           "PulseTrack",
		BundleID:          bundleIDPrefix() + ".pulsetrack",
		Platform:          "watchos",
		WatchProjectShape: "paired_ios_watch",
	}

	yml := generateProjectYAML(cfg)

	if !strings.Contains(yml, "WKCompanionAppBundleIdentifier: "+bundleIDPrefix()+".pulsetrack") {
		t.Fatalf("paired watch YAML missing WKCompanionAppBundleIdentifier for iOS companion bundle ID:\n%s", yml)
	}
	if !strings.Contains(yml, "WKRunsIndependentlyOfCompanionApp: true") {
		t.Error("paired watch YAML should preserve WKRunsIndependentlyOfCompanionApp")
	}
	checks := []string{
		"PulseTrackWatchExtension:",
		"type: watchkit2-extension",
		"target: PulseTrackWatchExtension",
		"NSExtensionPointIdentifier: com.apple.watchkit",
		"WKAppBundleIdentifier: " + bundleIDPrefix() + ".pulsetrack.watchkitapp",
	}
	for _, want := range checks {
		if !strings.Contains(yml, want) {
			t.Fatalf("paired watch YAML missing %q:\n%s", want, yml)
		}
	}
}

func TestGenerateProjectYAML_WithPackages(t *testing.T) {
	cfg := &ProjectConfig{
		AppName:  "TestApp",
		BundleID: bundleIDPrefix() + ".testapp",
		Packages: []PackageDep{
			{
				Name:       "Lottie",
				URL:        "https://github.com/airbnb/lottie-ios",
				MinVersion: "4.0.0",
			},
			{
				Name:       "SDWebImageSwiftUI",
				URL:        "https://github.com/nicklama/SDWebImageSwiftUI",
				MinVersion: "2.0.0",
				Products:   []string{"SDWebImageSwiftUI"},
			},
		},
	}

	yml := generateProjectYAML(cfg)

	// Check packages section exists
	checks := []string{
		"packages:",
		"Lottie:",
		"url: https://github.com/airbnb/lottie-ios",
		"minVersion: 4.0.0",
		"SDWebImageSwiftUI:",
		"url: https://github.com/nicklama/SDWebImageSwiftUI",
		"minVersion: 2.0.0",
		// Check package dependency in target
		"- package: Lottie",
		"- package: SDWebImageSwiftUI",
		// Ensure dependencies section exists
		"dependencies:",
	}
	for _, want := range checks {
		if !strings.Contains(yml, want) {
			t.Errorf("YAML missing %q:\n%s", want, yml)
		}
	}
}

func TestGenerateProjectYAML_WithoutPackages(t *testing.T) {
	cfg := &ProjectConfig{
		AppName:  "TestApp",
		BundleID: bundleIDPrefix() + ".testapp",
	}

	yml := generateProjectYAML(cfg)

	// No packages section should be present
	if strings.Contains(yml, "packages:") {
		t.Errorf("YAML should not contain packages section when no packages defined:\n%s", yml)
	}
}

func TestGenerateProjectYAMLWatchOnlyOmitsCompanionBundleIdentifier(t *testing.T) {
	cfg := &ProjectConfig{
		AppName:           "PulseTrack",
		BundleID:          bundleIDPrefix() + ".pulsetrack",
		Platform:          "watchos",
		WatchProjectShape: "watch_only",
	}

	yml := generateProjectYAML(cfg)

	if strings.Contains(yml, "WKCompanionAppBundleIdentifier") {
		t.Fatal("watch_only YAML should not include WKCompanionAppBundleIdentifier")
	}
	if !strings.Contains(yml, "WKWatchOnly: true") {
		t.Error("watch_only YAML should include WKWatchOnly")
	}
	checks := []string{
		"PulseTrackWatch:",
		"PulseTrackWatchExtension:",
		"type: application.watchapp2-container",
		"type: application.watchapp2",
		"type: watchkit2-extension",
		"target: PulseTrackWatch",
		"target: PulseTrackWatchExtension",
		"NSExtensionPointIdentifier: com.apple.watchkit",
		"WKAppBundleIdentifier: " + bundleIDPrefix() + ".pulsetrack.watchkitapp",
	}
	for _, want := range checks {
		if !strings.Contains(yml, want) {
			t.Fatalf("watch_only YAML missing %q:\n%s", want, yml)
		}
	}
}
