package screenshots

import (
	"image"
	_ "image/jpeg"
	_ "image/png"
	"log"
	"os"
	"path/filepath"
	"strings"
)

// UploadedScreenshot represents a screenshot file with its detected device type.
type UploadedScreenshot struct {
	Filename   string
	DeviceType string // e.g. "IPHONE_69", auto-detected from dimensions
}

// ScreenshotRequirements describes what device types are needed for submission.
type ScreenshotRequirements struct {
	DeviceFamily string   // "iphone", "ipad", "universal"
	Required     []string // e.g. ["IPHONE_69"] or ["IPHONE_69", "IPAD_PRO_13"]
}

// dimensionToDevice maps width×height to ASC device type.
// Both portrait and landscape orientations are included.
var dimensionToDevice = map[[2]int]string{
	// iPhone 6.9" (mandatory primary)
	{1320, 2868}: "IPHONE_69", {2868, 1320}: "IPHONE_69",
	{1290, 2796}: "IPHONE_69", {2796, 1290}: "IPHONE_69",
	{1260, 2736}: "IPHONE_69", {2736, 1260}: "IPHONE_69",
	// iPhone 6.5" (fallback if 6.9" missing)
	{1284, 2778}: "IPHONE_65", {2778, 1284}: "IPHONE_65",
	{1242, 2688}: "IPHONE_65", {2688, 1242}: "IPHONE_65",
	// iPhone 6.3"
	{1179, 2556}: "IPHONE_63", {2556, 1179}: "IPHONE_63",
	// iPad 13" (mandatory if iPad app)
	{2064, 2752}: "IPAD_PRO_13", {2752, 2064}: "IPAD_PRO_13",
	{2048, 2732}: "IPAD_PRO_13", {2732, 2048}: "IPAD_PRO_13",
}

// FindScreenshotDir checks standard screenshot directories in priority order
// and returns the first one containing PNG/JPEG files.
func FindScreenshotDir(projectDir string) string {
	candidates := []string{
		filepath.Join(projectDir, "screenshots", "auto"),
		filepath.Join(projectDir, "screenshots", "upload"),
		filepath.Join(projectDir, "screenshots", "framed"),
		filepath.Join(projectDir, "screenshots"),
	}
	for _, dir := range candidates {
		if hasImages(dir) {
			return dir
		}
	}
	return ""
}

// ListScreenshots reads a directory and returns screenshot info with detected device types.
func ListScreenshots(dir string) []UploadedScreenshot {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	var result []UploadedScreenshot
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		if !isImageFile(e.Name()) {
			continue
		}
		deviceType := detectDeviceType(filepath.Join(dir, e.Name()))
		result = append(result, UploadedScreenshot{
			Filename:   e.Name(),
			DeviceType: deviceType,
		})
	}
	return result
}

func detectDeviceType(path string) string {
	f, err := os.Open(path)
	if err != nil {
		log.Printf("[screenshots] cannot open %s: %v", path, err)
		return ""
	}
	defer f.Close()

	cfg, _, err := image.DecodeConfig(f)
	if err != nil {
		log.Printf("[screenshots] cannot decode %s: %v", path, err)
		return ""
	}

	key := [2]int{cfg.Width, cfg.Height}
	if dt, ok := dimensionToDevice[key]; ok {
		return dt
	}
	return ""
}

func hasImages(dir string) bool {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return false
	}
	for _, e := range entries {
		if !e.IsDir() && isImageFile(e.Name()) {
			return true
		}
	}
	return false
}

func isImageFile(name string) bool {
	lower := strings.ToLower(name)
	return strings.HasSuffix(lower, ".png") || strings.HasSuffix(lower, ".jpg") || strings.HasSuffix(lower, ".jpeg")
}

// RequirementsForFamily returns the mandatory screenshot device types for a given device family.
func RequirementsForFamily(deviceFamily string) ScreenshotRequirements {
	switch deviceFamily {
	case "ipad":
		return ScreenshotRequirements{DeviceFamily: "ipad", Required: []string{"IPAD_PRO_13"}}
	case "universal":
		return ScreenshotRequirements{DeviceFamily: "universal", Required: []string{"IPHONE_69", "IPAD_PRO_13"}}
	default: // "iphone"
		return ScreenshotRequirements{DeviceFamily: "iphone", Required: []string{"IPHONE_69"}}
	}
}

// ValidateScreenshots checks uploaded screenshots against requirements.
// Returns which required types are fulfilled and which are missing.
// For iPhone: accepts either IPHONE_69 or IPHONE_65 to satisfy the iPhone requirement.
func ValidateScreenshots(dir string, reqs ScreenshotRequirements) (fulfilled []string, missing []string) {
	screenshots := ListScreenshots(dir)
	found := map[string]bool{}
	for _, s := range screenshots {
		if s.DeviceType != "" {
			found[s.DeviceType] = true
		}
	}

	for _, req := range reqs.Required {
		switch req {
		case "IPHONE_69":
			// Accept IPHONE_65 as fallback for iPhone requirement
			if found["IPHONE_69"] || found["IPHONE_65"] {
				fulfilled = append(fulfilled, req)
			} else {
				missing = append(missing, req)
			}
		default:
			if found[req] {
				fulfilled = append(fulfilled, req)
			} else {
				missing = append(missing, req)
			}
		}
	}
	return fulfilled, missing
}
