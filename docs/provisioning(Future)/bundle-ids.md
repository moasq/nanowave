# Bundle ID & Capabilities Management

Bundle IDs uniquely identify your app across Apple's ecosystem. Capabilities (push notifications, HealthKit, etc.) must be enabled on the bundle ID before the app can use them.

## Registering a Bundle ID

Every app needs a unique bundle ID registered with Apple before you can create provisioning profiles for it.

### API Request

```
POST /v1/bundleIds
```

```go
package signing

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"strings"
)

// BundleIDPlatform represents the platform for a bundle ID.
type BundleIDPlatform string

const (
	PlatformIOS       BundleIDPlatform = "IOS"
	PlatformMacOS     BundleIDPlatform = "MAC_OS"
	PlatformUniversal BundleIDPlatform = "UNIVERSAL"
)

// CreateBundleIDRequest is the API request body.
type CreateBundleIDRequest struct {
	Data struct {
		Type       string `json:"type"`
		Attributes struct {
			Identifier string           `json:"identifier"` // e.g., "com.nanowave.myapp"
			Name       string           `json:"name"`       // Display name
			Platform   BundleIDPlatform `json:"platform"`
		} `json:"attributes"`
	} `json:"data"`
}

// BundleIDResponse is the API response.
type BundleIDResponse struct {
	Data struct {
		ID         string `json:"id"`
		Attributes struct {
			Identifier string `json:"identifier"`
			Name       string `json:"name"`
			Platform   string `json:"platform"`
		} `json:"attributes"`
	} `json:"data"`
}

// RegisterBundleID creates a new bundle ID in App Store Connect.
func (c *Client) RegisterBundleID(identifier, name string, platform BundleIDPlatform) (*BundleIDResponse, error) {
	reqBody := CreateBundleIDRequest{}
	reqBody.Data.Type = "bundleIds"
	reqBody.Data.Attributes.Identifier = identifier
	reqBody.Data.Attributes.Name = name
	reqBody.Data.Attributes.Platform = platform

	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	resp, err := c.Do("POST", "/bundleIds", bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, fmt.Errorf("failed to register bundle ID: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 201 {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API returned %d: %s", resp.StatusCode, string(body))
	}

	var bundleResp BundleIDResponse
	if err := json.NewDecoder(resp.Body).Decode(&bundleResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &bundleResp, nil
}
```

### Finding Existing Bundle IDs

```go
// ListBundleIDsResponse is the API response for listing bundle IDs.
type ListBundleIDsResponse struct {
	Data []struct {
		ID         string `json:"id"`
		Attributes struct {
			Identifier string `json:"identifier"`
			Name       string `json:"name"`
			Platform   string `json:"platform"`
		} `json:"attributes"`
	} `json:"data"`
}

// FindBundleID searches for a bundle ID by identifier.
func (c *Client) FindBundleID(identifier string) (*BundleIDResponse, error) {
	path := "/bundleIds?filter[identifier]=" + identifier

	resp, err := c.Do("GET", path, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to find bundle ID: %w", err)
	}
	defer resp.Body.Close()

	var listResp ListBundleIDsResponse
	if err := json.NewDecoder(resp.Body).Decode(&listResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	if len(listResp.Data) == 0 {
		return nil, nil // Not found
	}

	return &BundleIDResponse{Data: listResp.Data[0]}, nil
}
```

## Enabling Capabilities

After registering a bundle ID, enable the capabilities your app needs (push notifications, HealthKit, etc.).

### API Request

```
POST /v1/bundleIdCapabilities
```

### Capability Types

| Apple Capability Type | Description | Nanowave Permission/Entitlement |
|---|---|---|
| `PUSH_NOTIFICATIONS` | Push notifications | `notifications` permission |
| `HEALTHKIT` | HealthKit access | `healthkit` permission |
| `MAPS` | MapKit | `maps` permission |
| `IN_APP_PURCHASE` | In-App Purchase | `in-app-purchase` entitlement |
| `APPLE_PAY` | Apple Pay | `apple-pay` entitlement |
| `ASSOCIATED_DOMAINS` | Universal links | `associated-domains` entitlement |
| `SIRI` | SiriKit | `siri` entitlement |
| `NETWORK_EXTENSIONS` | VPN / content filter | `network-extensions` entitlement |
| `ACCESS_WIFI_INFORMATION` | Wi-Fi info | `wifi-info` entitlement |
| `APP_GROUPS` | Shared containers | `app-groups` entitlement |
| `ICLOUD` | iCloud / CloudKit | `icloud` entitlement |
| `GAME_CENTER` | Game Center | `game-center` entitlement |
| `WALLET` | Wallet / PassKit | `wallet` entitlement |

### Go Implementation

```go
// CapabilityType represents Apple capability identifiers.
type CapabilityType string

const (
	CapPushNotifications CapabilityType = "PUSH_NOTIFICATIONS"
	CapHealthKit         CapabilityType = "HEALTHKIT"
	CapMaps              CapabilityType = "MAPS"
	CapInAppPurchase     CapabilityType = "IN_APP_PURCHASE"
	CapApplePay          CapabilityType = "APPLE_PAY"
	CapAssociatedDomains CapabilityType = "ASSOCIATED_DOMAINS"
	CapSiri              CapabilityType = "SIRI"
	CapAppGroups         CapabilityType = "APP_GROUPS"
	CapICloud            CapabilityType = "ICLOUD"
	CapGameCenter        CapabilityType = "GAME_CENTER"
	CapWallet            CapabilityType = "WALLET"
)

// EnableCapabilityRequest is the API request body.
type EnableCapabilityRequest struct {
	Data struct {
		Type       string `json:"type"`
		Attributes struct {
			CapabilityType CapabilityType `json:"capabilityType"`
		} `json:"attributes"`
		Relationships struct {
			BundleID struct {
				Data struct {
					Type string `json:"type"`
					ID   string `json:"id"`
				} `json:"data"`
			} `json:"bundleId"`
		} `json:"relationships"`
	} `json:"data"`
}

// EnableCapability adds a capability to a bundle ID.
func (c *Client) EnableCapability(bundleIDResourceID string, capType CapabilityType) error {
	reqBody := EnableCapabilityRequest{}
	reqBody.Data.Type = "bundleIdCapabilities"
	reqBody.Data.Attributes.CapabilityType = capType
	reqBody.Data.Relationships.BundleID.Data.Type = "bundleIds"
	reqBody.Data.Relationships.BundleID.Data.ID = bundleIDResourceID

	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return fmt.Errorf("failed to marshal request: %w", err)
	}

	resp, err := c.Do("POST", "/bundleIdCapabilities", bytes.NewReader(bodyBytes))
	if err != nil {
		return fmt.Errorf("failed to enable capability: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 201 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("API returned %d: %s", resp.StatusCode, string(body))
	}

	return nil
}
```

### Mapping Nanowave Config to Capabilities

The `project_config.json` contains `permissions` and `entitlements` arrays. Map these to Apple capability types:

```go
// NanowaveCapabilityMap maps nanowave permission/entitlement names to Apple capability types.
var NanowaveCapabilityMap = map[string]CapabilityType{
	// Permissions
	"notifications": CapPushNotifications,
	"healthkit":     CapHealthKit,
	"maps":          CapMaps,

	// Entitlements
	"in-app-purchase":    CapInAppPurchase,
	"apple-pay":          CapApplePay,
	"associated-domains": CapAssociatedDomains,
	"siri":               CapSiri,
	"app-groups":         CapAppGroups,
	"icloud":             CapICloud,
	"game-center":        CapGameCenter,
	"wallet":             CapWallet,
}

// EnableCapabilitiesFromConfig reads the project config and enables matching capabilities.
func (c *Client) EnableCapabilitiesFromConfig(bundleIDResourceID string, permissions []string, entitlements []string) error {
	all := append(permissions, entitlements...)

	for _, name := range all {
		capType, ok := NanowaveCapabilityMap[name]
		if !ok {
			continue // No matching Apple capability
		}

		if err := c.EnableCapability(bundleIDResourceID, capType); err != nil {
			return fmt.Errorf("failed to enable %s: %w", name, err)
		}
	}

	return nil
}
```

## Bundle ID Naming

Nanowave uses the convention `com.nanowave.<appName>`:

```go
// BundleIDFromAppName generates a bundle ID from an app name.
func BundleIDFromAppName(appName string) string {
	// Lowercase, remove spaces, keep alphanumeric and hyphens
	sanitized := strings.Map(func(r rune) rune {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' {
			return r
		}
		if r >= 'A' && r <= 'Z' {
			return r + 32 // lowercase
		}
		return -1 // drop
	}, appName)

	return "com.nanowave." + sanitized
}
```

## Extension Bundle IDs

Widget extensions, live activities, and other app extensions need their own bundle IDs as children of the main app:

```
com.nanowave.myapp                   ← Main app
com.nanowave.myapp.widgets           ← Widget extension
com.nanowave.myapp.liveactivity      ← Live activity extension
```

Each extension bundle ID must be registered separately and may need its own capabilities.
