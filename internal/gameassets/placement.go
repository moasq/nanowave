package gameassets

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
)

// PlaceImageAsset copies srcPath into Assets.xcassets/<assetName>.imageset/
// and writes the corresponding Contents.json. It converts non-PNG images
// to PNG using macOS sips. Returns the relative path to the placed file.
func PlaceImageAsset(projectDir, appName, assetName, srcPath string) (string, error) {
	assetName = SanitizeAssetName(assetName)
	imagesetDir := filepath.Join(projectDir, appName, "Assets.xcassets", assetName+".imageset")
	if err := os.MkdirAll(imagesetDir, 0o755); err != nil {
		return "", fmt.Errorf("failed to create imageset directory: %w", err)
	}

	dstName := assetName + ".png"
	dstPath := filepath.Join(imagesetDir, dstName)

	if err := convertToPNG(srcPath, dstPath); err != nil {
		return "", fmt.Errorf("failed to convert image: %w", err)
	}

	contentsJSON := ImageSetContentsJSON(dstName)
	contentsPath := filepath.Join(imagesetDir, "Contents.json")
	if err := os.WriteFile(contentsPath, contentsJSON, 0o644); err != nil {
		return "", fmt.Errorf("failed to write Contents.json: %w", err)
	}

	relPath := filepath.Join(appName, "Assets.xcassets", assetName+".imageset", dstName)
	return relPath, nil
}

// PlaceSoundAsset copies srcPath into <appName>/Sounds/<assetName>.<ext>.
// Returns the relative path to the placed file.
func PlaceSoundAsset(projectDir, appName, assetName, srcPath string) (string, error) {
	assetName = SanitizeAssetName(assetName)
	soundsDir := filepath.Join(projectDir, appName, "Sounds")
	if err := os.MkdirAll(soundsDir, 0o755); err != nil {
		return "", fmt.Errorf("failed to create Sounds directory: %w", err)
	}

	ext := filepath.Ext(srcPath)
	if ext == "" {
		ext = ".wav"
	}
	dstName := assetName + ext
	dstPath := filepath.Join(soundsDir, dstName)

	if err := copyFile(srcPath, dstPath); err != nil {
		return "", fmt.Errorf("failed to copy sound file: %w", err)
	}

	relPath := filepath.Join(appName, "Sounds", dstName)
	return relPath, nil
}

// PlaceModelAsset copies srcPath into <appName>/Models/<assetName>.<ext>.
// SceneKit loads USDZ/OBJ/DAE/SCN files natively — no conversion needed.
// Returns the relative path to the placed file.
func PlaceModelAsset(projectDir, appName, assetName, srcPath string) (string, error) {
	assetName = SanitizeAssetName(assetName)
	modelsDir := filepath.Join(projectDir, appName, "Models")
	if err := os.MkdirAll(modelsDir, 0o755); err != nil {
		return "", fmt.Errorf("failed to create Models directory: %w", err)
	}

	ext := filepath.Ext(srcPath)
	if ext == "" {
		ext = ".usdz"
	}
	dstName := assetName + ext
	dstPath := filepath.Join(modelsDir, dstName)

	if err := copyFile(srcPath, dstPath); err != nil {
		return "", fmt.Errorf("failed to copy model file: %w", err)
	}

	relPath := filepath.Join(appName, "Models", dstName)
	return relPath, nil
}

// ImageSetContentsJSON returns a standard Xcode imageset Contents.json
// for a universal idiom with the given filename.
func ImageSetContentsJSON(filename string) []byte {
	contents := map[string]any{
		"images": []map[string]string{
			{
				"filename": filename,
				"idiom":    "universal",
			},
		},
		"info": map[string]any{
			"version": 1,
			"author":  "xcode",
		},
	}
	data, _ := json.MarshalIndent(contents, "", "  ")
	return data
}

// SanitizeAssetName converts a name to a valid Xcode asset catalog name.
// Lowercase, spaces/underscores to hyphens, strip non-alphanumeric except hyphens.
func SanitizeAssetName(name string) string {
	name = strings.ToLower(strings.TrimSpace(name))
	name = strings.ReplaceAll(name, " ", "-")
	name = strings.ReplaceAll(name, "_", "-")
	re := regexp.MustCompile(`[^a-z0-9\-]`)
	name = re.ReplaceAllString(name, "")
	// Collapse multiple hyphens
	re = regexp.MustCompile(`-{2,}`)
	name = re.ReplaceAllString(name, "-")
	name = strings.Trim(name, "-")
	if name == "" {
		name = "asset"
	}
	return name
}

// convertToPNG converts an image file to PNG using macOS sips.
// If the source is already PNG, it copies directly.
func convertToPNG(src, dst string) error {
	ext := strings.ToLower(filepath.Ext(src))
	if ext == ".png" {
		return copyFile(src, dst)
	}

	// Strip xattrs that can cause sips to reject the file
	_ = exec.Command("xattr", "-c", src).Run()

	cmd := exec.Command("sips", "-s", "format", "png", src, "--out", dst)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("sips conversion failed: %s: %w", string(out), err)
	}

	info, err := os.Stat(dst)
	if err != nil || info.Size() == 0 {
		return fmt.Errorf("sips output file missing or empty after conversion")
	}
	return nil
}

func copyFile(src, dst string) error {
	if src == dst {
		return nil
	}
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()
	_, err = io.Copy(out, in)
	return err
}
