# Provisioning Profile Automation

Provisioning profiles are the final piece that ties everything together: they bind a **certificate** + **bundle ID** + **devices** into a single file that Xcode uses to sign your app.

## Profile Types

| API Type | Use Case | Requires Devices? |
|---|---|---|
| `IOS_APP_DEVELOPMENT` | Run on registered test devices | Yes |
| `IOS_APP_STORE` | App Store & TestFlight submission | No |
| `IOS_APP_ADHOC` | Beta distribution to specific devices | Yes |
| `IOS_APP_INHOUSE` | Enterprise distribution (requires Enterprise account) | No |

### When to Use Each

- **Building for your device during development**: `IOS_APP_DEVELOPMENT`
- **Distributing to beta testers via TestFlight**: `IOS_APP_STORE`
- **Distributing to testers without TestFlight**: `IOS_APP_ADHOC`
- **Submitting to the App Store**: `IOS_APP_STORE`

## Creating a Profile

### API Request

```
POST /v1/profiles
```

```go
package signing

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// ProfileType represents the provisioning profile type.
type ProfileType string

const (
	ProfileDevelopment ProfileType = "IOS_APP_DEVELOPMENT"
	ProfileAppStore    ProfileType = "IOS_APP_STORE"
	ProfileAdHoc       ProfileType = "IOS_APP_ADHOC"
)

// CreateProfileRequest is the API request body.
type CreateProfileRequest struct {
	Data struct {
		Type       string `json:"type"`
		Attributes struct {
			Name        string      `json:"name"`
			ProfileType ProfileType `json:"profileType"`
		} `json:"attributes"`
		Relationships struct {
			BundleID struct {
				Data struct {
					Type string `json:"type"`
					ID   string `json:"id"`
				} `json:"data"`
			} `json:"bundleId"`
			Certificates struct {
				Data []struct {
					Type string `json:"type"`
					ID   string `json:"id"`
				} `json:"data"`
			} `json:"certificates"`
			Devices struct {
				Data []struct {
					Type string `json:"type"`
					ID   string `json:"id"`
				} `json:"data"`
			} `json:"devices"`
		} `json:"relationships"`
	} `json:"data"`
}

// ProfileResponse is the API response.
type ProfileResponse struct {
	Data struct {
		ID         string `json:"id"`
		Attributes struct {
			Name           string `json:"name"`
			ProfileType    string `json:"profileType"`
			ProfileState   string `json:"profileState"`   // ACTIVE or INVALID
			ProfileContent string `json:"profileContent"` // Base64-encoded .mobileprovision
			ExpirationDate string `json:"expirationDate"`
			UUID           string `json:"uuid"`
		} `json:"attributes"`
	} `json:"data"`
}

// CreateDevelopmentProfile creates a development provisioning profile.
func (c *Client) CreateDevelopmentProfile(
	name string,
	bundleIDResourceID string,
	certificateIDs []string,
	deviceIDs []string,
) (*ProfileResponse, error) {
	return c.createProfile(name, ProfileDevelopment, bundleIDResourceID, certificateIDs, deviceIDs)
}

// CreateAppStoreProfile creates an App Store provisioning profile.
func (c *Client) CreateAppStoreProfile(
	name string,
	bundleIDResourceID string,
	certificateIDs []string,
) (*ProfileResponse, error) {
	return c.createProfile(name, ProfileAppStore, bundleIDResourceID, certificateIDs, nil)
}

// CreateAdHocProfile creates an ad hoc provisioning profile.
func (c *Client) CreateAdHocProfile(
	name string,
	bundleIDResourceID string,
	certificateIDs []string,
	deviceIDs []string,
) (*ProfileResponse, error) {
	return c.createProfile(name, ProfileAdHoc, bundleIDResourceID, certificateIDs, deviceIDs)
}

func (c *Client) createProfile(
	name string,
	profileType ProfileType,
	bundleIDResourceID string,
	certificateIDs []string,
	deviceIDs []string,
) (*ProfileResponse, error) {
	reqBody := CreateProfileRequest{}
	reqBody.Data.Type = "profiles"
	reqBody.Data.Attributes.Name = name
	reqBody.Data.Attributes.ProfileType = profileType

	// Bundle ID relationship
	reqBody.Data.Relationships.BundleID.Data.Type = "bundleIds"
	reqBody.Data.Relationships.BundleID.Data.ID = bundleIDResourceID

	// Certificate relationships
	for _, certID := range certificateIDs {
		reqBody.Data.Relationships.Certificates.Data = append(
			reqBody.Data.Relationships.Certificates.Data,
			struct {
				Type string `json:"type"`
				ID   string `json:"id"`
			}{Type: "certificates", ID: certID},
		)
	}

	// Device relationships (optional — not needed for App Store profiles)
	for _, deviceID := range deviceIDs {
		reqBody.Data.Relationships.Devices.Data = append(
			reqBody.Data.Relationships.Devices.Data,
			struct {
				Type string `json:"type"`
				ID   string `json:"id"`
			}{Type: "devices", ID: deviceID},
		)
	}

	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	resp, err := c.Do("POST", "/profiles", bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, fmt.Errorf("failed to create profile: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 201 {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API returned %d: %s", resp.StatusCode, string(body))
	}

	var profileResp ProfileResponse
	if err := json.NewDecoder(resp.Body).Decode(&profileResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &profileResp, nil
}
```

## Installing a Profile

Provisioning profiles must be installed to a specific directory on macOS for Xcode to find them.

```go
// InstallProfile decodes and writes a provisioning profile to the system location.
// The file is named by UUID to match Xcode's convention.
func InstallProfile(profileResp *ProfileResponse) (string, error) {
	// Decode base64 profile content
	profileData, err := base64.StdEncoding.DecodeString(profileResp.Data.Attributes.ProfileContent)
	if err != nil {
		return "", fmt.Errorf("failed to decode profile: %w", err)
	}

	// Profiles go in ~/Library/MobileDevice/Provisioning Profiles/
	profileDir := filepath.Join(os.Getenv("HOME"), "Library", "MobileDevice", "Provisioning Profiles")
	if err := os.MkdirAll(profileDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create profile directory: %w", err)
	}

	// Name by UUID (Xcode convention)
	profilePath := filepath.Join(profileDir, profileResp.Data.Attributes.UUID+".mobileprovision")
	if err := os.WriteFile(profilePath, profileData, 0644); err != nil {
		return "", fmt.Errorf("failed to write profile: %w", err)
	}

	return profilePath, nil
}
```

### Profile Directory

```
~/Library/MobileDevice/Provisioning Profiles/
├── abc12345-6789-0abc-def0-123456789abc.mobileprovision
├── def98765-4321-0fed-cba0-987654321fed.mobileprovision
└── ...
```

Each file is named `<UUID>.mobileprovision`. Xcode discovers profiles from this directory automatically.

## Listing Existing Profiles

```go
// ListProfilesResponse is the API response for listing profiles.
type ListProfilesResponse struct {
	Data []struct {
		ID         string `json:"id"`
		Attributes struct {
			Name           string `json:"name"`
			ProfileType    string `json:"profileType"`
			ProfileState   string `json:"profileState"`
			ExpirationDate string `json:"expirationDate"`
			UUID           string `json:"uuid"`
		} `json:"attributes"`
	} `json:"data"`
}

// ListProfiles returns all provisioning profiles, optionally filtered.
func (c *Client) ListProfiles(profileType ProfileType) (*ListProfilesResponse, error) {
	path := "/profiles"
	if profileType != "" {
		path += "?filter[profileType]=" + string(profileType)
	}

	resp, err := c.Do("GET", path, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to list profiles: %w", err)
	}
	defer resp.Body.Close()

	var listResp ListProfilesResponse
	if err := json.NewDecoder(resp.Body).Decode(&listResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &listResp, nil
}
```

## Deleting a Profile

Profiles should be deleted and recreated when:
- A new device is added (development/ad hoc)
- A certificate is renewed
- Capabilities change

```go
// DeleteProfile removes a provisioning profile.
func (c *Client) DeleteProfile(profileID string) error {
	resp, err := c.Do("DELETE", "/profiles/"+profileID, nil)
	if err != nil {
		return fmt.Errorf("failed to delete profile: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 204 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("API returned %d: %s", resp.StatusCode, string(body))
	}

	return nil
}
```

## Profile Renewal Strategy

Provisioning profiles expire after **1 year**. Strategy:

1. **Check expiration** before each build
2. If expiring within 30 days, regenerate
3. Delete old profile from Apple and local disk
4. Create new profile with same parameters
5. Install the new profile

```go
import "time"

// NeedsRenewal returns true if the profile expires within the threshold.
func NeedsRenewal(expirationDate string, threshold time.Duration) (bool, error) {
	expiry, err := time.Parse("2006-01-02T15:04:05.000-0700", expirationDate)
	if err != nil {
		return false, fmt.Errorf("failed to parse expiration date: %w", err)
	}
	return time.Until(expiry) < threshold, nil
}
```

## Profile Invalidation

Profiles become `INVALID` when:
- The signing certificate is revoked or expires
- A device in the profile is removed
- The bundle ID capabilities change
- The profile itself expires

Always check `profileState` before using a profile. If `INVALID`, delete and recreate it.

## Extension Profiles

Each app extension (widgets, live activities, etc.) needs its own provisioning profile with its own bundle ID:

```
Main app:     com.nanowave.myapp          → Profile: "MyApp Development"
Widget:       com.nanowave.myapp.widgets  → Profile: "MyApp Widgets Development"
```

When setting up signing for a project with extensions:
1. Register all bundle IDs (main + each extension)
2. Enable capabilities on each bundle ID
3. Create a profile for each bundle ID
4. Install all profiles
