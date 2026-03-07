package orchestration

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/moasq/nanowave/internal/screenshots"
	"github.com/moasq/nanowave/internal/terminal"
)

// checkScreenshots checks whether the project has existing screenshots and validates them.
// Returns whether screenshots were found, the count, the directory path, and validation results.
func (p *Pipeline) checkScreenshots(projectDir, deviceFamily string) (found bool, count int, dir string, fulfilled []string, missing []string) {
	dir = screenshots.FindScreenshotDir(projectDir)
	if dir == "" {
		return false, 0, "", nil, nil
	}
	list := screenshots.ListScreenshots(dir)
	if len(list) == 0 {
		return false, 0, "", nil, nil
	}
	log.Printf("[asc][screenshots] found %d screenshots in %s", len(list), dir)

	reqs := screenshots.RequirementsForFamily(deviceFamily)
	fulfilled, missing = screenshots.ValidateScreenshots(dir, reqs)
	return true, len(list), dir, fulfilled, missing
}

// offerScreenshotOptions shows a 3-option picker for screenshot acquisition.
// Returns the screenshot directory path and optional capture result.
func (p *Pipeline) offerScreenshotOptions(ctx context.Context, projectDir, deviceFamily string) (dir string, captureResult *screenshots.CaptureResult) {
	log.Printf("[asc] offering screenshot options (deviceFamily=%s)", deviceFamily)

	picked := terminal.Pick("App screenshots", []terminal.PickerOption{
		{Label: "Automatic", Desc: "Build and capture from simulator (recommended)"},
		{Label: "Custom", Desc: "Upload your own screenshots in browser"},
		{Label: "Skip", Desc: "Continue without screenshots"},
	}, "")

	reqs := screenshots.RequirementsForFamily(deviceFamily)

	switch picked {
	case "Automatic":
		autoDir := filepath.Join(projectDir, "screenshots", "auto")
		if err := os.MkdirAll(autoDir, 0o755); err != nil {
			log.Printf("[asc] failed to create auto screenshot dir: %v", err)
			terminal.Error("Could not create screenshots directory.")
			return "", nil
		}

		spinner := terminal.NewSpinner("Preparing automatic capture...")
		spinner.Start()
		progress := func(step string) {
			spinner.Update(step)
		}

		cr, err := screenshots.CaptureFromSimulator(ctx, projectDir, reqs, progress)
		spinner.Stop()

		if err != nil {
			log.Printf("[asc] automatic capture failed: %v", err)
			terminal.Warning(fmt.Sprintf("Automatic capture failed: %v", err))
			terminal.Info("Falling back to custom upload...")
			return p.offerCustomUpload(ctx, projectDir, reqs)
		}

		// Show summary
		var parts []string
		iPhoneCount := 0
		iPadCount := 0
		for _, s := range cr.Screenshots {
			switch {
			case strings.HasPrefix(s.DeviceType, "IPHONE"):
				iPhoneCount++
			case strings.HasPrefix(s.DeviceType, "IPAD"):
				iPadCount++
			}
		}
		if iPhoneCount > 0 {
			parts = append(parts, fmt.Sprintf("%d iPhone", iPhoneCount))
		}
		if iPadCount > 0 {
			parts = append(parts, fmt.Sprintf("%d iPad", iPadCount))
		}
		if len(parts) > 0 {
			terminal.Success(fmt.Sprintf("Captured %s screenshot(s)", strings.Join(parts, ", ")))
		}
		if len(cr.Errors) > 0 {
			for _, e := range cr.Errors {
				terminal.Warning(e)
			}
		}

		return cr.Dir, cr

	case "Custom":
		return p.offerCustomUpload(ctx, projectDir, reqs)

	default: // "Skip"
		return "", nil
	}
}

func (p *Pipeline) offerCustomUpload(ctx context.Context, projectDir string, reqs screenshots.ScreenshotRequirements) (string, *screenshots.CaptureResult) {
	uploadDir := filepath.Join(projectDir, "screenshots", "upload")
	if err := os.MkdirAll(uploadDir, 0o755); err != nil {
		log.Printf("[asc] failed to create screenshot dir: %v", err)
		terminal.Error("Could not create screenshots directory.")
		return "", nil
	}

	if screenshots.RunUploadServer(ctx, uploadDir, reqs) {
		return uploadDir, nil
	}
	return "", nil
}
