package orchestration

import (
	"context"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/moasq/nanowave/internal/icons"
	"github.com/moasq/nanowave/internal/terminal"
)

// checkAppIcon checks whether the project already has a valid app icon
// and generates all required sizes if a source icon exists.
// Returns whether an icon was found and the number of PNG sizes generated.
func (p *Pipeline) checkAppIcon(projectDir, platform string) (found bool, sizeCount int) {
	appIconDir, err := icons.FindAppIconDir(projectDir)
	if err != nil {
		log.Printf("[asc] icon dir not found: %v", err)
		return false, 0
	}
	log.Printf("[asc][icon] found appiconset at %s", appIconDir)

	if !icons.HasExisting(appIconDir) {
		log.Printf("[asc] no icon found in %s", appIconDir)
		return false, 0
	}

	log.Printf("[asc][icon] icon already exists in %s", appIconDir)
	if src := icons.FindSourceIcon(appIconDir); src != "" {
		log.Printf("[asc][icon] source icon for resizing: %s", src)
		if err := icons.UpdateContentsJSON(appIconDir, filepath.Base(src), platform); err != nil {
			log.Printf("[asc][icon] FAILED to generate icon sizes: %v", err)
		}
	} else {
		log.Printf("[asc][icon] WARNING: no source icon found for resizing in %s", appIconDir)
	}

	entries, _ := os.ReadDir(appIconDir)
	count := 0
	for _, e := range entries {
		if strings.HasSuffix(strings.ToLower(e.Name()), ".png") {
			count++
		}
	}
	log.Printf("[asc][icon] icon check complete: %d PNG files", count)
	return true, count
}

// offerIconUpload opens a local browser page where the user can drag-and-drop
// an icon. Called only when checkAppIcon returns found=false.
// Returns true if icon was set.
func (p *Pipeline) offerIconUpload(ctx context.Context, projectDir, platform string) bool {
	log.Printf("[asc] no icon found, offering upload UI")
	terminal.Warning("No app icon found.")

	picked := terminal.Pick("App icon", []terminal.PickerOption{
		{Label: "Upload icon", Desc: "Open browser to drag-and-drop your icon"},
		{Label: "Skip", Desc: "Continue without an icon"},
	}, "")

	if picked != "Upload icon" {
		return false
	}

	appIconDir, err := icons.FindAppIconDir(projectDir)
	if err != nil {
		log.Printf("[asc] icon dir not found for upload: %v", err)
		terminal.Error("Could not find app icon directory.")
		return false
	}

	// Start local server for icon upload
	return icons.RunUploadServer(ctx, appIconDir, platform)
}
