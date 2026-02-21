# Device Registration

To install a development build on a physical iOS device, the device must be registered with your Apple Developer account and included in the provisioning profile.

## Registering a Device

### API Request

```
POST /v1/devices
```

```go
package signing

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os/exec"
	"regexp"
	"strings"
)

// DevicePlatform represents the device platform.
type DevicePlatform string

const (
	DevicePlatformIOS   DevicePlatform = "IOS"
	DevicePlatformMacOS DevicePlatform = "MAC_OS"
)

// RegisterDeviceRequest is the API request body.
type RegisterDeviceRequest struct {
	Data struct {
		Type       string `json:"type"`
		Attributes struct {
			Name     string         `json:"name"`     // Human-readable name
			UDID     string         `json:"udid"`     // Device UDID
			Platform DevicePlatform `json:"platform"`
		} `json:"attributes"`
	} `json:"data"`
}

// DeviceResponse is the API response.
type DeviceResponse struct {
	Data struct {
		ID         string `json:"id"`
		Attributes struct {
			Name        string `json:"name"`
			UDID        string `json:"udid"`
			Platform    string `json:"platform"`
			Status      string `json:"status"`      // ENABLED or DISABLED
			DeviceClass string `json:"deviceClass"` // APPLE_WATCH, IPAD, IPHONE, etc.
		} `json:"attributes"`
	} `json:"data"`
}

// RegisterDevice adds a device to your Apple Developer account.
func (c *Client) RegisterDevice(name, udid string, platform DevicePlatform) (*DeviceResponse, error) {
	reqBody := RegisterDeviceRequest{}
	reqBody.Data.Type = "devices"
	reqBody.Data.Attributes.Name = name
	reqBody.Data.Attributes.UDID = udid
	reqBody.Data.Attributes.Platform = platform

	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	resp, err := c.Do("POST", "/devices", bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, fmt.Errorf("failed to register device: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 201 {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API returned %d: %s", resp.StatusCode, string(body))
	}

	var deviceResp DeviceResponse
	if err := json.NewDecoder(resp.Body).Decode(&deviceResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &deviceResp, nil
}
```

### Listing Registered Devices

```go
// ListDevicesResponse is the API response for listing devices.
type ListDevicesResponse struct {
	Data []struct {
		ID         string `json:"id"`
		Attributes struct {
			Name        string `json:"name"`
			UDID        string `json:"udid"`
			Platform    string `json:"platform"`
			Status      string `json:"status"`
			DeviceClass string `json:"deviceClass"`
		} `json:"attributes"`
	} `json:"data"`
}

// ListDevices returns all registered devices.
func (c *Client) ListDevices() (*ListDevicesResponse, error) {
	resp, err := c.Do("GET", "/devices", nil)
	if err != nil {
		return nil, fmt.Errorf("failed to list devices: %w", err)
	}
	defer resp.Body.Close()

	var listResp ListDevicesResponse
	if err := json.NewDecoder(resp.Body).Decode(&listResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &listResp, nil
}
```

## Getting Device UDID

### Method 1: Connected USB Device (Recommended)

Use `xcrun xctrace` to find connected iOS devices:

```go
// ConnectedDevice holds info about a USB-connected iOS device.
type ConnectedDevice struct {
	Name string
	UDID string
}

// FindConnectedDevices returns iOS devices connected via USB.
func FindConnectedDevices() ([]ConnectedDevice, error) {
	// Use xcrun xctrace to list devices
	out, err := exec.Command("xcrun", "xctrace", "list", "devices").Output()
	if err != nil {
		return nil, fmt.Errorf("failed to list devices: %w", err)
	}

	var devices []ConnectedDevice
	// Pattern: "Device Name (OS Version) (UDID)"
	re := regexp.MustCompile(`^(.+?)\s+\([\d.]+\)\s+\(([0-9a-fA-F-]{25,})\)$`)

	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(line)
		matches := re.FindStringSubmatch(line)
		if len(matches) == 3 {
			devices = append(devices, ConnectedDevice{
				Name: strings.TrimSpace(matches[1]),
				UDID: matches[2],
			})
		}
	}

	return devices, nil
}
```

### Method 2: From Finder/iTunes

1. Connect the iOS device to your Mac via USB
2. Open **Finder** (macOS Catalina+) or **iTunes** (older macOS)
3. Select the device
4. Click the device info area repeatedly until the **UDID** is shown
5. Right-click to copy

### Method 3: From Xcode

1. Open **Xcode → Window → Devices and Simulators**
2. Select the connected device
3. The **Identifier** field shows the UDID

### Method 4: From Device Settings

1. On the iOS device: **Settings → General → About**
2. Find the **UDID** field (may need to scroll)
3. Long-press to copy

## Device Limits

| Account Type | Max Devices Per Year |
|---|---|
| Individual | 100 per device class |
| Organization | 100 per device class |

Device classes: iPhone, iPad, Apple Watch, Apple TV, Mac.

**Important**: The 100-device limit resets annually at your membership renewal date. Devices cannot be removed and re-added to free up slots within the same year — removed devices still count against the limit until renewal.

## Device Status

Devices can be `ENABLED` or `DISABLED`:

```go
// DisableDevice sets a device status to DISABLED. It remains registered
// but won't be included in new provisioning profiles.
func (c *Client) DisableDevice(deviceID string) error {
	reqBody := map[string]any{
		"data": map[string]any{
			"type": "devices",
			"id":   deviceID,
			"attributes": map[string]any{
				"status": "DISABLED",
			},
		},
	}

	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return fmt.Errorf("failed to marshal request: %w", err)
	}

	resp, err := c.Do("PATCH", "/devices/"+deviceID, bytes.NewReader(bodyBytes))
	if err != nil {
		return fmt.Errorf("failed to disable device: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("API returned %d: %s", resp.StatusCode, string(body))
	}

	return nil
}
```

## When Devices Are Needed

| Profile Type | Requires Devices? |
|---|---|
| `IOS_APP_DEVELOPMENT` | Yes — specific device UDIDs |
| `IOS_APP_ADHOC` | Yes — specific device UDIDs |
| `IOS_APP_STORE` | No |
| `IOS_APP_INHOUSE` | No (Enterprise only) |

For Nanowave's typical use case (running on a personal device during development), you need to:
1. Discover the device UDID
2. Register it
3. Include it in a development provisioning profile
