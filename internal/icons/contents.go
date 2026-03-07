package icons

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
)

// UpdateContentsJSON generates all required icon sizes from the source image
// using macOS sips, then writes a complete Contents.json referencing every size.
func UpdateContentsJSON(appIconDir, iconFilename, platform string) error {
	sourcePath := filepath.Join(appIconDir, iconFilename)
	log.Printf("[asc][icon] UpdateContentsJSON: dir=%s source=%s platform=%s", appIconDir, iconFilename, platform)

	// Verify source exists and get its dimensions
	srcInfo, srcErr := os.Stat(sourcePath)
	if srcErr != nil {
		log.Printf("[asc][icon] ERROR: source icon not found: %v", srcErr)
		return fmt.Errorf("source icon not found: %w", srcErr)
	}
	log.Printf("[asc][icon] source icon: %s (%d bytes)", sourcePath, srcInfo.Size())

	type imageEntry struct {
		Filename string `json:"filename,omitempty"`
		Idiom    string `json:"idiom"`
		Platform string `json:"platform,omitempty"`
		Size     string `json:"size"`
		Scale    string `json:"scale,omitempty"`
	}
	type contentsJSON struct {
		Images []imageEntry `json:"images"`
		Info   struct {
			Version int    `json:"version"`
			Author  string `json:"author"`
		} `json:"info"`
	}

	var entries []imageEntry

	switch strings.ToLower(platform) {
	case "ios", "":
		specs := IOSSpecs()
		log.Printf("[asc][icon] generating %d icon sizes for iOS", len(specs))
		for _, spec := range specs {
			destPath := filepath.Join(appIconDir, spec.Filename)
			// Copy source for 1024, resize for others
			if spec.Pixels == 1024 {
				if err := CopyFile(sourcePath, destPath); err != nil {
					return fmt.Errorf("failed to copy icon for %s: %w", spec.Size, err)
				}
				log.Printf("[asc][icon]   copied: %s (1024x1024)", spec.Filename)
			} else {
				if err := Resize(sourcePath, destPath, spec.Pixels); err != nil {
					return fmt.Errorf("failed to resize icon to %dx%d: %w", spec.Pixels, spec.Pixels, err)
				}
				log.Printf("[asc][icon]   resized: %s (%dx%d)", spec.Filename, spec.Pixels, spec.Pixels)
			}
			entries = append(entries, imageEntry{
				Filename: spec.Filename,
				Idiom:    spec.Idiom,
				Size:     spec.Size,
				Scale:    spec.Scale,
			})
		}
	case "watchos":
		// watchOS: universal 1024x1024 entry is sufficient
		entries = append(entries, imageEntry{
			Filename: iconFilename,
			Idiom:    "universal",
			Platform: "watchos",
			Size:     "1024x1024",
		})
	case "tvos":
		entries = append(entries, imageEntry{
			Filename: iconFilename,
			Idiom:    "tv",
			Platform: "tvos",
			Size:     "1280x768",
		})
	case "macos":
		entries = append(entries, imageEntry{
			Filename: iconFilename,
			Idiom:    "mac",
			Size:     "1024x1024",
			Scale:    "1x",
		})
	case "visionos":
		entries = append(entries, imageEntry{
			Filename: iconFilename,
			Idiom:    "universal",
			Platform: "xros",
			Size:     "1024x1024",
		})
	}

	contents := contentsJSON{
		Images: entries,
	}
	contents.Info.Version = 1
	contents.Info.Author = "xcode"

	log.Printf("[asc][icon] Contents.json will have %d image entries", len(entries))
	for i, e := range entries {
		log.Printf("[asc][icon]   entry[%d]: filename=%s idiom=%s size=%s scale=%s", i, e.Filename, e.Idiom, e.Size, e.Scale)
	}

	data, err := json.MarshalIndent(contents, "", "  ")
	if err != nil {
		return err
	}
	outPath := filepath.Join(appIconDir, "Contents.json")
	if writeErr := os.WriteFile(outPath, append(data, '\n'), 0o644); writeErr != nil {
		log.Printf("[asc][icon] ERROR writing Contents.json: %v", writeErr)
		return writeErr
	}
	log.Printf("[asc][icon] Contents.json written successfully to %s (%d bytes)", outPath, len(data))
	return nil
}
