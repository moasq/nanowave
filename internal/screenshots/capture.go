package screenshots

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// CaptureResult holds the outcome of an automatic capture run.
type CaptureResult struct {
	Dir         string               // directory where screenshots were saved
	Screenshots []UploadedScreenshot  // what was captured
	Errors      []string             // any capture failures
	SimUDIDs    map[string]string    // device type -> UDID (kept booted for agent use)
}

// CaptureFromSimulator builds the app, boots simulator(s), and captures initial screenshots.
// Simulators are left running so the ASC agent can capture additional screens via AXe.
func CaptureFromSimulator(ctx context.Context, projectDir string, reqs ScreenshotRequirements, progress func(step string)) (*CaptureResult, error) {
	result := &CaptureResult{
		Dir:      filepath.Join(projectDir, "screenshots", "auto"),
		SimUDIDs: make(map[string]string),
	}

	if err := os.MkdirAll(result.Dir, 0o755); err != nil {
		return nil, fmt.Errorf("create screenshot dir: %w", err)
	}

	// Read project config
	configPath := filepath.Join(projectDir, "project_config.json")
	configData, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("read project_config.json: %w", err)
	}
	var cfg struct {
		AppName  string `json:"app_name"`
		BundleID string `json:"bundle_id"`
	}
	if err := json.Unmarshal(configData, &cfg); err != nil {
		return nil, fmt.Errorf("parse project_config.json: %w", err)
	}
	if cfg.BundleID == "" {
		return nil, fmt.Errorf("bundle_id not found in project_config.json")
	}

	// Find .xcodeproj
	xcodeProj, err := findXcodeProj(projectDir)
	if err != nil {
		return nil, err
	}

	// Determine scheme name (typically the app name)
	scheme := cfg.AppName
	if scheme == "" {
		scheme = strings.TrimSuffix(filepath.Base(xcodeProj), ".xcodeproj")
	}

	// Determine which simulators are needed
	type simTarget struct {
		deviceType string // e.g. "IPHONE_69", "IPAD_PRO_13"
		label      string // e.g. "iPhone", "iPad"
	}
	var targets []simTarget
	for _, req := range reqs.Required {
		switch req {
		case "IPHONE_69":
			targets = append(targets, simTarget{"IPHONE_69", "iPhone"})
		case "IPAD_PRO_13":
			targets = append(targets, simTarget{"IPAD_PRO_13", "iPad"})
		}
	}

	derivedData := filepath.Join(projectDir, ".derivedData-screenshots")
	defer os.RemoveAll(derivedData)

	for _, target := range targets {
		if ctx.Err() != nil {
			return result, ctx.Err()
		}

		// Find best simulator
		progress(fmt.Sprintf("Finding %s simulator...", target.label))
		udid, simName, err := findBestSimulator(target.deviceType)
		if err != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("%s: %v", target.label, err))
			log.Printf("[screenshots] failed to find %s simulator: %v", target.label, err)
			continue
		}
		log.Printf("[screenshots] selected %s simulator: %s (%s)", target.label, simName, udid)
		result.SimUDIDs[target.deviceType] = udid

		// Boot simulator
		progress(fmt.Sprintf("Booting %s simulator...", target.label))
		if err := bootSimulator(udid); err != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("boot %s: %v", target.label, err))
			log.Printf("[screenshots] failed to boot %s: %v", target.label, err)
			continue
		}

		// Build app
		progress(fmt.Sprintf("Building app for %s simulator...", target.label))
		if err := buildForSimulator(ctx, xcodeProj, scheme, udid, derivedData); err != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("build %s: %v", target.label, err))
			log.Printf("[screenshots] build failed for %s: %v", target.label, err)
			continue
		}

		// Find and install .app
		appPath, err := findBuiltApp(derivedData)
		if err != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("find app %s: %v", target.label, err))
			log.Printf("[screenshots] failed to find built app for %s: %v", target.label, err)
			continue
		}

		progress(fmt.Sprintf("Installing app on %s simulator...", target.label))
		if err := installOnSimulator(udid, appPath); err != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("install %s: %v", target.label, err))
			log.Printf("[screenshots] failed to install on %s: %v", target.label, err)
			continue
		}

		// Launch app
		progress(fmt.Sprintf("Launching app on %s simulator...", target.label))
		if err := launchOnSimulator(udid, cfg.BundleID); err != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("launch %s: %v", target.label, err))
			log.Printf("[screenshots] failed to launch on %s: %v", target.label, err)
			continue
		}

		// Wait for app to render
		time.Sleep(3 * time.Second)

		// Capture screenshot
		progress(fmt.Sprintf("Capturing %s screenshot...", target.label))
		outputPath := filepath.Join(result.Dir, fmt.Sprintf("%s_launch.png", strings.ToLower(target.label)))
		if err := captureScreenshot(udid, outputPath); err != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("capture %s: %v", target.label, err))
			log.Printf("[screenshots] failed to capture %s: %v", target.label, err)
			continue
		}

		dt := detectDeviceType(outputPath)
		result.Screenshots = append(result.Screenshots, UploadedScreenshot{
			Filename:   filepath.Base(outputPath),
			DeviceType: dt,
		})
		log.Printf("[screenshots] captured %s: %s (device type: %s)", target.label, outputPath, dt)
	}

	if len(result.Screenshots) == 0 && len(result.Errors) > 0 {
		return result, fmt.Errorf("all captures failed: %s", strings.Join(result.Errors, "; "))
	}

	return result, nil
}

func findXcodeProj(projectDir string) (string, error) {
	entries, err := os.ReadDir(projectDir)
	if err != nil {
		return "", fmt.Errorf("read project dir: %w", err)
	}
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".xcodeproj") {
			return filepath.Join(projectDir, e.Name()), nil
		}
	}
	return "", fmt.Errorf("no .xcodeproj found in %s", projectDir)
}

// findBestSimulator finds the best available simulator for the given device type.
func findBestSimulator(deviceType string) (udid, name string, err error) {
	out, err := exec.Command("xcrun", "simctl", "list", "devices", "available", "-j").Output()
	if err != nil {
		return "", "", fmt.Errorf("list simulators: %w", err)
	}

	var result struct {
		Devices map[string][]struct {
			Name                 string `json:"name"`
			UDID                 string `json:"udid"`
			IsAvailable          bool   `json:"isAvailable"`
			DeviceTypeIdentifier string `json:"deviceTypeIdentifier"`
		} `json:"devices"`
	}
	if err := json.Unmarshal(out, &result); err != nil {
		return "", "", fmt.Errorf("parse simulator list: %w", err)
	}

	var matchFunc func(string) bool
	switch deviceType {
	case "IPHONE_69":
		matchFunc = func(dtID string) bool {
			lower := strings.ToLower(dtID)
			return strings.Contains(lower, "iphone") && strings.Contains(lower, "pro-max")
		}
	case "IPAD_PRO_13":
		matchFunc = func(dtID string) bool {
			lower := strings.ToLower(dtID)
			return strings.Contains(lower, "ipad-pro") && strings.Contains(lower, "13")
		}
	default:
		return "", "", fmt.Errorf("unsupported device type: %s", deviceType)
	}

	type candidate struct {
		name    string
		udid    string
		runtime string
	}
	var candidates []candidate

	for runtime, devs := range result.Devices {
		if !strings.Contains(runtime, "iOS") {
			continue
		}
		for _, d := range devs {
			if !d.IsAvailable {
				continue
			}
			if matchFunc(d.DeviceTypeIdentifier) {
				candidates = append(candidates, candidate{
					name:    d.Name,
					udid:    d.UDID,
					runtime: runtime,
				})
			}
		}
	}

	if len(candidates) == 0 {
		return "", "", fmt.Errorf("no simulator found for device type %s", deviceType)
	}

	// Pick the one with the newest runtime (lexicographic sort works for versioned runtime strings)
	best := candidates[0]
	for _, c := range candidates[1:] {
		if c.runtime > best.runtime {
			best = c
		}
	}

	return best.udid, best.name, nil
}

func bootSimulator(udid string) error {
	out, err := exec.Command("xcrun", "simctl", "boot", udid).CombinedOutput()
	if err != nil {
		// Ignore "already booted" errors
		text := strings.ToLower(string(out) + " " + err.Error())
		if strings.Contains(text, "already booted") || strings.Contains(text, "current state: booted") {
			return nil
		}
		return fmt.Errorf("%s: %s", err, out)
	}
	return nil
}

func buildForSimulator(ctx context.Context, xcodeProj, scheme, udid, derivedData string) error {
	cmd := exec.CommandContext(ctx, "xcodebuild",
		"-project", xcodeProj,
		"-scheme", scheme,
		"-destination", fmt.Sprintf("platform=iOS Simulator,id=%s", udid),
		"-derivedDataPath", derivedData,
		"-quiet",
		"build",
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("xcodebuild: %w\n%s", err, out)
	}
	return nil
}

func findBuiltApp(derivedData string) (string, error) {
	productsDir := filepath.Join(derivedData, "Build", "Products")
	entries, err := os.ReadDir(productsDir)
	if err != nil {
		return "", fmt.Errorf("read products dir: %w", err)
	}

	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		// Look in Debug-iphonesimulator/ or similar directories
		subDir := filepath.Join(productsDir, e.Name())
		subEntries, err := os.ReadDir(subDir)
		if err != nil {
			continue
		}
		for _, se := range subEntries {
			if strings.HasSuffix(se.Name(), ".app") {
				return filepath.Join(subDir, se.Name()), nil
			}
		}
	}
	return "", fmt.Errorf("no .app found in %s", productsDir)
}

func installOnSimulator(udid, appPath string) error {
	out, err := exec.Command("xcrun", "simctl", "install", udid, appPath).CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s: %s", err, out)
	}
	return nil
}

func launchOnSimulator(udid, bundleID string) error {
	out, err := exec.Command("xcrun", "simctl", "launch", udid, bundleID).CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s: %s", err, out)
	}
	return nil
}

func captureScreenshot(udid, outputPath string) error {
	out, err := exec.Command("xcrun", "simctl", "io", udid, "screenshot", outputPath).CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s: %s", err, out)
	}
	return nil
}
