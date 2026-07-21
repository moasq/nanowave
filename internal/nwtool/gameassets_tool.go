package nwtool

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/moasq/nanowave/internal/gameassets"
)

func registerGameAssetTools(r *Registry) {
	r.Register(downloadAssetTool())
}

func downloadAssetTool() *Tool {
	return &Tool{
		Name: "nw_download_asset",
		Description: `Download an asset from a URL and place it in the Xcode project.
For images (sprite/background/texture): placed in Assets.xcassets as a named image set.
  Use via SKTexture(imageNamed: "name") in SpriteKit or Image("name") in SwiftUI.
  Non-PNG images are automatically converted to PNG.
For sounds: placed in <AppName>/Sounds/ directory.
  Use via Bundle.main.url(forResource: "name", withExtension: "wav").
For 3D models (model): placed in <AppName>/Models/ directory.
  Use via SCNScene(named: "name.usdz") in SceneKit.
  Supports USDZ, OBJ, DAE, SCN formats — no conversion needed.
Supports PNG, JPEG, SVG, WAV, MP3, AIFF, CAF, USDZ, OBJ, DAE, SCN. Only HTTPS URLs are allowed.`,
		InputSchema: json.RawMessage(`{
  "type": "object",
  "properties": {
    "project_dir": {"type": "string", "description": "Absolute path to the project directory"},
    "app_name":    {"type": "string", "description": "PascalCase app name"},
    "url":         {"type": "string", "description": "HTTPS URL to download the asset from"},
    "asset_name":  {"type": "string", "description": "Name for the asset in the project (e.g. 'paddle', 'ball', 'hit-sound', 'car-model')"},
    "asset_kind":  {"type": "string", "enum": ["sprite", "background", "texture", "sound", "model"], "description": "How the asset will be used. sprite/background/texture go to Assets.xcassets. sound goes to Sounds/. model goes to Models/ (USDZ/OBJ/DAE/SCN for SceneKit)."}
  },
  "required": ["project_dir", "app_name", "url", "asset_name", "asset_kind"]
}`),
		Handler: handleDownloadAsset,
	}
}

func handleDownloadAsset(ctx context.Context, input json.RawMessage) (json.RawMessage, error) {
	var in struct {
		ProjectDir string `json:"project_dir"`
		AppName    string `json:"app_name"`
		URL        string `json:"url"`
		AssetName  string `json:"asset_name"`
		AssetKind  string `json:"asset_kind"`
	}
	if err := json.Unmarshal(input, &in); err != nil {
		return jsonError(fmt.Sprintf("invalid input: %v", err))
	}

	if in.ProjectDir == "" || in.AppName == "" || in.URL == "" || in.AssetName == "" || in.AssetKind == "" {
		return jsonError("all fields are required: project_dir, app_name, url, asset_name, asset_kind")
	}

	// Download to a temp file.
	tmpFile, err := os.CreateTemp("", "nw-asset-*")
	if err != nil {
		return jsonError(fmt.Sprintf("failed to create temp file: %v", err))
	}
	tmpPath := tmpFile.Name()
	tmpFile.Close()
	defer os.Remove(tmpPath)

	if err := gameassets.DownloadFile(ctx, in.URL, tmpPath); err != nil {
		return jsonError(fmt.Sprintf("download failed: %v", err))
	}

	sanitizedName := gameassets.SanitizeAssetName(in.AssetName)

	switch in.AssetKind {
	case "sprite", "background", "texture":
		relPath, err := gameassets.PlaceImageAsset(in.ProjectDir, in.AppName, sanitizedName, tmpPath)
		if err != nil {
			return jsonError(fmt.Sprintf("failed to place image asset: %v", err))
		}
		return jsonOK(map[string]any{
			"success":         true,
			"file_path":       relPath,
			"asset_name":      sanitizedName,
			"usage_spritekit": fmt.Sprintf("SKTexture(imageNamed: %q)", sanitizedName),
			"usage_swiftui":   fmt.Sprintf("Image(%q)", sanitizedName),
		})

	case "sound":
		relPath, err := gameassets.PlaceSoundAsset(in.ProjectDir, in.AppName, sanitizedName, tmpPath)
		if err != nil {
			return jsonError(fmt.Sprintf("failed to place sound asset: %v", err))
		}
		ext := filepath.Ext(relPath)
		if ext != "" {
			ext = ext[1:] // strip leading dot
		}
		return jsonOK(map[string]any{
			"success":    true,
			"file_path":  relPath,
			"asset_name": sanitizedName,
			"usage_spritekit": fmt.Sprintf(
				"SKAction.playSoundFileNamed(%q, waitForCompletion: false)",
				sanitizedName+"."+ext,
			),
			"usage_bundle": fmt.Sprintf(
				"Bundle.main.url(forResource: %q, withExtension: %q)",
				sanitizedName, ext,
			),
		})

	case "model":
		relPath, err := gameassets.PlaceModelAsset(in.ProjectDir, in.AppName, sanitizedName, tmpPath)
		if err != nil {
			return jsonError(fmt.Sprintf("failed to place model asset: %v", err))
		}
		ext := filepath.Ext(relPath)
		modelFile := sanitizedName + ext
		return jsonOK(map[string]any{
			"success":         true,
			"file_path":       relPath,
			"asset_name":      sanitizedName,
			"usage_scenekit":  fmt.Sprintf("SCNScene(named: %q)", modelFile),
			"usage_modelio":   fmt.Sprintf("MDLAsset(url: Bundle.main.url(forResource: %q, withExtension: %q)!)", sanitizedName, ext[1:]),
		})

	default:
		return jsonError(fmt.Sprintf("unknown asset_kind %q — use sprite, background, texture, sound, or model", in.AssetKind))
	}
}
