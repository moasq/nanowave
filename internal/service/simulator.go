package service

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"sort"
	"strings"
)

// rankSimulator scores a device based on its type identifier.
// Higher scores are preferred. This avoids hardcoding device marketing names
// which change across Xcode versions.
func rankSimulator(deviceTypeID, family, platform string) int {
	if platform == "macos" {
		return -1 // macOS has no simulators
	}

	lower := strings.ToLower(deviceTypeID)

	if platform == "visionos" {
		if !strings.Contains(lower, "apple-vision") {
			return -1
		}
		switch {
		case strings.Contains(lower, "4k"):
			return 100
		default:
			return 60
		}
	}

	if platform == "tvos" {
		if !strings.Contains(lower, "apple-tv") {
			return -1
		}
		switch {
		case strings.Contains(lower, "4k"):
			return 100
		default:
			return 60
		}
	}

	if platform == "watchos" {
		if !strings.Contains(lower, "watch") {
			return -1
		}
		switch {
		case strings.Contains(lower, "ultra"):
			return 100
		case strings.Contains(lower, "series"):
			return 80
		case strings.Contains(lower, "se"):
			return 50
		default:
			return 60
		}
	}

	switch family {
	case "ipad":
		if !strings.Contains(lower, "ipad") {
			return -1
		}
		switch {
		case strings.Contains(lower, "ipad-pro") && strings.Contains(lower, "13"):
			return 100
		case strings.Contains(lower, "ipad-pro"):
			return 95
		case strings.Contains(lower, "ipad-air") && strings.Contains(lower, "13"):
			return 85
		case strings.Contains(lower, "ipad-air"):
			return 80
		case strings.Contains(lower, "ipad-mini"):
			return 60
		default:
			return 70
		}

	case "universal":
		// Accept both iPhone and iPad, prefer pro models
		isIPhone := strings.Contains(lower, "iphone")
		isIPad := strings.Contains(lower, "ipad")
		if !isIPhone && !isIPad {
			return -1
		}
		switch {
		case isIPhone && strings.Contains(lower, "pro-max"):
			return 100
		case isIPhone && strings.Contains(lower, "pro"):
			return 95
		case isIPad && strings.Contains(lower, "ipad-pro"):
			return 90
		case isIPhone && !strings.Contains(lower, "se"):
			return 80
		case isIPad:
			return 75
		default:
			return 50
		}

	default: // "iphone"
		if !strings.Contains(lower, "iphone") {
			return -1
		}
		switch {
		case strings.Contains(lower, "pro-max"):
			return 100
		case strings.Contains(lower, "pro"):
			return 90
		case strings.Contains(lower, "plus"):
			return 80
		case strings.Contains(lower, "air"):
			return 70
		case strings.Contains(lower, "se"):
			return 50
		default:
			return 75 // standard iPhone models
		}
	}
}

// SimulatorDevice represents an available simulator.
type SimulatorDevice struct {
	Name         string
	UDID         string
	Runtime      string // e.g. "iOS 18.1"
	DeviceTypeID string // e.g. "com.apple.CoreSimulator.SimDeviceType.iPhone-16-Pro"
}

// detectDefaultSimulator picks the best available simulator for the current device family.
// It ranks all available devices dynamically using their device type identifiers
// rather than hardcoded marketing names, which change across Xcode versions.
func (s *Service) detectDefaultSimulator() string {
	family := s.currentDeviceFamily()
	platform := s.currentPlatform()
	devices, err := s.ListSimulators()
	if err != nil || len(devices) == 0 {
		return "Simulator"
	}

	bestIdx := 0
	bestScore := rankSimulator(devices[0].DeviceTypeID, family, platform)
	for i := 1; i < len(devices); i++ {
		score := rankSimulator(devices[i].DeviceTypeID, family, platform)
		if score > bestScore {
			bestScore = score
			bestIdx = i
		}
	}
	return devices[bestIdx].Name
}

// ListSimulators returns available simulator devices for the current platform and device family.
func (s *Service) ListSimulators() ([]SimulatorDevice, error) {
	// macOS has no simulators — apps run natively on the Mac.
	if s.currentPlatform() == "macos" {
		return nil, nil
	}

	out, err := exec.Command("xcrun", "simctl", "list", "devices", "available", "-j").Output()
	if err != nil {
		return nil, fmt.Errorf("failed to list simulators: %w", err)
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
		return nil, fmt.Errorf("failed to parse simulator list: %w", err)
	}

	platform := s.currentPlatform()
	family := s.currentDeviceFamily()

	// Filter runtimes by platform
	runtimeFilter := "iOS"
	switch platform {
	case "watchos":
		runtimeFilter = "watchOS"
	case "tvos":
		runtimeFilter = "tvOS"
	case "visionos":
		runtimeFilter = "xrOS"
	}

	var devices []SimulatorDevice
	for runtime, devs := range result.Devices {
		if !strings.Contains(runtime, runtimeFilter) {
			continue
		}
		runtimeName := parseRuntimeName(runtime)
		for _, d := range devs {
			if !d.IsAvailable {
				continue
			}
			// Use device type identifier for filtering instead of marketing names
			score := rankSimulator(d.DeviceTypeIdentifier, family, platform)
			if score < 0 {
				continue // not relevant for this family/platform
			}
			devices = append(devices, SimulatorDevice{
				Name:         d.Name,
				UDID:         d.UDID,
				Runtime:      runtimeName,
				DeviceTypeID: d.DeviceTypeIdentifier,
			})
		}
	}

	// Sort: newest runtime first, then by rank (best device first), then by name
	sort.Slice(devices, func(i, j int) bool {
		if devices[i].Runtime != devices[j].Runtime {
			return devices[i].Runtime > devices[j].Runtime
		}
		ri := rankSimulator(devices[i].DeviceTypeID, family, platform)
		rj := rankSimulator(devices[j].DeviceTypeID, family, platform)
		if ri != rj {
			return ri > rj
		}
		return devices[i].Name < devices[j].Name
	})

	// Deduplicate by name — keep only the newest runtime version
	seen := map[string]bool{}
	var unique []SimulatorDevice
	for _, d := range devices {
		if seen[d.Name] {
			continue
		}
		seen[d.Name] = true
		unique = append(unique, d)
	}

	return unique, nil
}

// resolveSimulatorUDID looks up the UDID for a simulator by name.
// Returns "" if the simulator cannot be found.
func (s *Service) resolveSimulatorUDID(name string) string {
	devices, err := s.ListSimulators()
	if err != nil {
		return ""
	}
	for _, d := range devices {
		if d.Name == name {
			return d.UDID
		}
	}
	return ""
}

// parseRuntimeName converts "com.apple.CoreSimulator.SimRuntime.iOS-18-1" to "iOS 18.1".
func parseRuntimeName(runtime string) string {
	// Extract the part after the last "SimRuntime."
	parts := strings.Split(runtime, "SimRuntime.")
	if len(parts) < 2 {
		return runtime
	}
	name := parts[1]
	// "iOS-18-1" → "iOS 18.1"
	name = strings.Replace(name, "-", " ", 1)
	name = strings.ReplaceAll(name, "-", ".")
	return name
}

func isAlreadyBootedSimError(err error, output []byte) bool {
	if err == nil {
		return false
	}
	text := strings.ToLower(err.Error())
	if len(output) > 0 {
		text += " " + strings.ToLower(string(output))
	}
	return strings.Contains(text, "already booted") ||
		strings.Contains(text, "current state: booted") ||
		strings.Contains(text, "unable to boot device in current state: booted")
}
