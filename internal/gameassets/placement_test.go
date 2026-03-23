package gameassets

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestSanitizeAssetName(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"paddle", "paddle"},
		{"Ping Pong Ball", "ping-pong-ball"},
		{"my_asset_name", "my-asset-name"},
		{"  spaces  ", "spaces"},
		{"UPPER", "upper"},
		{"special!@#chars", "specialchars"},
		{"multi---hyphens", "multi-hyphens"},
		{"-leading-trailing-", "leading-trailing"},
		{"", "asset"},
		{"   ", "asset"},
		{"hello world 123", "hello-world-123"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := SanitizeAssetName(tt.input)
			if got != tt.want {
				t.Errorf("SanitizeAssetName(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestImageSetContentsJSON(t *testing.T) {
	data := ImageSetContentsJSON("paddle.png")

	var contents struct {
		Images []struct {
			Filename string `json:"filename"`
			Idiom    string `json:"idiom"`
		} `json:"images"`
		Info struct {
			Version int    `json:"version"`
			Author  string `json:"author"`
		} `json:"info"`
	}

	if err := json.Unmarshal(data, &contents); err != nil {
		t.Fatalf("failed to unmarshal Contents.json: %v", err)
	}

	if len(contents.Images) != 1 {
		t.Fatalf("expected 1 image entry, got %d", len(contents.Images))
	}
	if contents.Images[0].Filename != "paddle.png" {
		t.Errorf("filename = %q, want %q", contents.Images[0].Filename, "paddle.png")
	}
	if contents.Images[0].Idiom != "universal" {
		t.Errorf("idiom = %q, want %q", contents.Images[0].Idiom, "universal")
	}
	if contents.Info.Version != 1 {
		t.Errorf("version = %d, want 1", contents.Info.Version)
	}
	if contents.Info.Author != "xcode" {
		t.Errorf("author = %q, want %q", contents.Info.Author, "xcode")
	}
}

func TestPlaceImageAsset(t *testing.T) {
	tmpDir := t.TempDir()
	projectDir := filepath.Join(tmpDir, "project")
	appName := "MyGame"

	// Create a fake PNG source file.
	srcDir := filepath.Join(tmpDir, "src")
	os.MkdirAll(srcDir, 0o755)
	srcPath := filepath.Join(srcDir, "test.png")
	os.WriteFile(srcPath, []byte("fake-png-data"), 0o644)

	relPath, err := PlaceImageAsset(projectDir, appName, "paddle", srcPath)
	if err != nil {
		t.Fatalf("PlaceImageAsset failed: %v", err)
	}

	expectedRel := filepath.Join(appName, "Assets.xcassets", "paddle.imageset", "paddle.png")
	if relPath != expectedRel {
		t.Errorf("relPath = %q, want %q", relPath, expectedRel)
	}

	// Verify the file was placed.
	placedPath := filepath.Join(projectDir, relPath)
	if _, err := os.Stat(placedPath); err != nil {
		t.Errorf("placed file does not exist: %v", err)
	}

	// Verify Contents.json was written.
	contentsPath := filepath.Join(projectDir, appName, "Assets.xcassets", "paddle.imageset", "Contents.json")
	contentsData, err := os.ReadFile(contentsPath)
	if err != nil {
		t.Fatalf("Contents.json not found: %v", err)
	}
	if len(contentsData) == 0 {
		t.Error("Contents.json is empty")
	}
}

func TestPlaceModelAsset(t *testing.T) {
	tmpDir := t.TempDir()
	projectDir := filepath.Join(tmpDir, "project")
	appName := "MyGame"

	// Create a fake USDZ source file.
	srcDir := filepath.Join(tmpDir, "src")
	os.MkdirAll(srcDir, 0o755)
	srcPath := filepath.Join(srcDir, "car.usdz")
	os.WriteFile(srcPath, []byte("fake-usdz-data"), 0o644)

	relPath, err := PlaceModelAsset(projectDir, appName, "race-car", srcPath)
	if err != nil {
		t.Fatalf("PlaceModelAsset failed: %v", err)
	}

	expectedRel := filepath.Join(appName, "Models", "race-car.usdz")
	if relPath != expectedRel {
		t.Errorf("relPath = %q, want %q", relPath, expectedRel)
	}

	// Verify the file was placed.
	placedPath := filepath.Join(projectDir, relPath)
	if _, err := os.Stat(placedPath); err != nil {
		t.Errorf("placed file does not exist: %v", err)
	}

	// Verify the content was copied correctly.
	data, err := os.ReadFile(placedPath)
	if err != nil {
		t.Fatalf("failed to read placed file: %v", err)
	}
	if string(data) != "fake-usdz-data" {
		t.Errorf("placed file content = %q, want %q", string(data), "fake-usdz-data")
	}
}

func TestPlaceModelAssetDefaultExt(t *testing.T) {
	tmpDir := t.TempDir()
	projectDir := filepath.Join(tmpDir, "project")
	appName := "MyGame"

	// Create a source file without extension.
	srcDir := filepath.Join(tmpDir, "src")
	os.MkdirAll(srcDir, 0o755)
	srcPath := filepath.Join(srcDir, "model")
	os.WriteFile(srcPath, []byte("fake-model-data"), 0o644)

	relPath, err := PlaceModelAsset(projectDir, appName, "spaceship", srcPath)
	if err != nil {
		t.Fatalf("PlaceModelAsset failed: %v", err)
	}

	expectedRel := filepath.Join(appName, "Models", "spaceship.usdz")
	if relPath != expectedRel {
		t.Errorf("relPath = %q, want %q", relPath, expectedRel)
	}
}

func TestPlaceSoundAsset(t *testing.T) {
	tmpDir := t.TempDir()
	projectDir := filepath.Join(tmpDir, "project")
	appName := "MyGame"

	// Create a fake WAV source file.
	srcDir := filepath.Join(tmpDir, "src")
	os.MkdirAll(srcDir, 0o755)
	srcPath := filepath.Join(srcDir, "hit.wav")
	os.WriteFile(srcPath, []byte("fake-wav-data"), 0o644)

	relPath, err := PlaceSoundAsset(projectDir, appName, "hit-sound", srcPath)
	if err != nil {
		t.Fatalf("PlaceSoundAsset failed: %v", err)
	}

	expectedRel := filepath.Join(appName, "Sounds", "hit-sound.wav")
	if relPath != expectedRel {
		t.Errorf("relPath = %q, want %q", relPath, expectedRel)
	}

	// Verify the file was placed.
	placedPath := filepath.Join(projectDir, relPath)
	if _, err := os.Stat(placedPath); err != nil {
		t.Errorf("placed file does not exist: %v", err)
	}
}
