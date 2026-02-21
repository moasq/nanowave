# CLI Integration Spec

This document specifies how provisioning automation integrates into the Nanowave CLI. It covers proposed commands, storage design, integration points in existing code, and the new package structure.

## Proposed CLI Commands

### `nanowave signing setup`

Interactive setup wizard that walks through the entire provisioning flow:

```
$ nanowave signing setup

🔑 App Store Connect API Key Setup
  Key ID: ABC123DEFG
  Issuer ID: 57246542-96fe-1a63-e053-0824d011072a
  Path to .p8 file: ~/.nanowave/keys/AuthKey_ABC123DEFG.p8

📋 Certificate
  ✓ Found existing IOS_DEVELOPMENT certificate (expires 2027-02-15)

📱 Device Registration
  Found connected device: iPhone 15 Pro (UDID: 00008030-...)
  ✓ Device already registered

📦 Bundle ID: com.nanowave.myapp
  ✓ Already registered
  Enabling capabilities: PUSH_NOTIFICATIONS, HEALTHKIT

📄 Provisioning Profile
  ✓ Created "MyApp Development" profile
  ✓ Installed to ~/Library/MobileDevice/Provisioning Profiles/

✅ Signing configured! Run 'nanowave build --device' to build for your device.
```

### `nanowave signing status`

Show current signing configuration:

```
$ nanowave signing status

API Key:     ABC123DEFG (Admin)
Certificate: Apple Development: John Doe (TEAMID) — expires 2027-02-15
Bundle ID:   com.nanowave.myapp — capabilities: PUSH_NOTIFICATIONS, HEALTHKIT
Device:      iPhone 15 Pro (00008030-...)
Profile:     MyApp Development — ACTIVE — expires 2027-02-15
Team ID:     TEAMID
```

### `nanowave signing reset`

Remove all local signing configuration:

```
$ nanowave signing reset

This will remove:
  - API key configuration (not the .p8 file itself)
  - Local signing config (.nanowave/signing.json)
  - Installed provisioning profiles for this project

Continue? (y/n): y
✓ Signing configuration removed.
```

## Storage Design

### `.nanowave/signing.json`

Per-project signing configuration stored alongside the project:

```json
{
  "api_key": {
    "key_id": "ABC123DEFG",
    "issuer_id": "57246542-96fe-1a63-e053-0824d011072a",
    "private_key_path": "~/.nanowave/keys/AuthKey_ABC123DEFG.p8"
  },
  "team_id": "TEAMID",
  "certificate": {
    "id": "cert-resource-id",
    "type": "IOS_DEVELOPMENT",
    "serial_number": "ABC123",
    "expiration_date": "2027-02-15T00:00:00.000+0000",
    "identity_hash": "ABC123DEF456..."
  },
  "bundle_id": {
    "id": "bundleid-resource-id",
    "identifier": "com.nanowave.myapp",
    "capabilities": ["PUSH_NOTIFICATIONS", "HEALTHKIT"]
  },
  "devices": [
    {
      "id": "device-resource-id",
      "name": "iPhone 15 Pro",
      "udid": "00008030-..."
    }
  ],
  "profiles": {
    "development": {
      "id": "profile-resource-id",
      "uuid": "abc12345-...",
      "expiration_date": "2027-02-15T00:00:00.000+0000",
      "local_path": "~/Library/MobileDevice/Provisioning Profiles/abc12345-....mobileprovision"
    },
    "app_store": null
  },
  "private_key_path": ".nanowave/signing/cert_private_key.pem"
}
```

### Go Type Definition

```go
package signing

// Config holds the complete signing configuration for a project.
type Config struct {
	APIKey     APIKeyConfig     `json:"api_key"`
	TeamID     string           `json:"team_id"`
	Certificate CertConfig     `json:"certificate"`
	BundleID   BundleIDConfig   `json:"bundle_id"`
	Devices    []DeviceConfig   `json:"devices"`
	Profiles   ProfilesConfig   `json:"profiles"`
	PrivateKeyPath string       `json:"private_key_path"`
}

type CertConfig struct {
	ID             string `json:"id"`
	Type           string `json:"type"`
	SerialNumber   string `json:"serial_number"`
	ExpirationDate string `json:"expiration_date"`
	IdentityHash   string `json:"identity_hash"`
}

type BundleIDConfig struct {
	ID           string   `json:"id"`
	Identifier   string   `json:"identifier"`
	Capabilities []string `json:"capabilities"`
}

type DeviceConfig struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	UDID string `json:"udid"`
}

type ProfilesConfig struct {
	Development *ProfileConfig `json:"development"`
	AppStore    *ProfileConfig `json:"app_store"`
}

type ProfileConfig struct {
	ID             string `json:"id"`
	UUID           string `json:"uuid"`
	ExpirationDate string `json:"expiration_date"`
	LocalPath      string `json:"local_path"`
}
```

## Integration Points in Existing Code

### 1. `cli/internal/orchestration/xcodegen.go:83` — CODE_SIGN_STYLE

**Current**: Hardcodes `CODE_SIGN_STYLE: Automatic`

**Change**: When signing config exists, switch to manual signing:

```go
// Before (line 83)
b.WriteString("        CODE_SIGN_STYLE: Automatic\n")

// After
if signingConfig != nil {
    b.WriteString("        CODE_SIGN_STYLE: Manual\n")
    b.WriteString(fmt.Sprintf("        DEVELOPMENT_TEAM: %s\n", signingConfig.TeamID))
    b.WriteString(fmt.Sprintf("        PROVISIONING_PROFILE_SPECIFIER: %s\n", signingConfig.Profiles.Development.UUID))
    b.WriteString(fmt.Sprintf("        CODE_SIGN_IDENTITY: Apple Development\n"))
} else {
    b.WriteString("        CODE_SIGN_STYLE: Automatic\n")
}
```

Same change at line 167 for extension targets.

### 2. `cli/internal/xcodegenserver/config.go:137` — MCP Server Config

**Current**: Same hardcoded `CODE_SIGN_STYLE: Automatic`

**Change**: Read signing config and inject manual signing settings. The `BuildSettings` map on `ProjectConfig` (line 20) already supports arbitrary build settings — signing settings can flow through this mechanism:

```go
// In project_config.json:
{
  "build_settings": {
    "CODE_SIGN_STYLE": "Manual",
    "DEVELOPMENT_TEAM": "TEAMID",
    "PROVISIONING_PROFILE_SPECIFIER": "abc12345-..."
  }
}
```

### 3. `cli/internal/service/service.go:372` — xcodebuild Flags

**Current**: No signing flags passed to xcodebuild

**Change**: When building for a device (not simulator), add signing flags:

```go
// Before (line 372)
buildCmd := exec.CommandContext(ctx, "xcodebuild",
    "-project", xcodeprojName,
    "-scheme", scheme,
    "-destination", destination,
    "-quiet",
    "build",
)

// After (for device builds)
args := []string{
    "-project", xcodeprojName,
    "-scheme", scheme,
    "-destination", destination,
    "-quiet",
}

if signingConfig != nil && isDeviceBuild {
    args = append(args,
        fmt.Sprintf("CODE_SIGN_STYLE=Manual"),
        fmt.Sprintf("DEVELOPMENT_TEAM=%s", signingConfig.TeamID),
        fmt.Sprintf("CODE_SIGN_IDENTITY=Apple Development"),
        fmt.Sprintf("PROVISIONING_PROFILE_SPECIFIER=%s", signingConfig.Profiles.Development.UUID),
    )
}

args = append(args, "build")
buildCmd := exec.CommandContext(ctx, "xcodebuild", args...)
```

### 4. `cli/internal/storage/project.go` — SigningConfig Field

**Current**: No signing fields

**Change**: Add `SigningConfig` to the project struct:

```go
type Project struct {
    // ... existing fields ...
    SigningConfig *signing.Config `json:"signing_config,omitempty"`
}
```

Or reference an external file:

```go
type Project struct {
    // ... existing fields ...
    SigningConfigPath string `json:"signing_config_path,omitempty"`
}
```

### 5. `cli/internal/orchestration/pipeline.go` — Build Phase Prompts

**Current**: xcodebuild commands in prompts don't include signing flags

**Change**: When signing config exists, include signing flags in the build commands emitted to Claude Code, so the LLM-driven build pipeline also uses correct signing.

## New Go Package Design

```
cli/internal/signing/
├── config.go          # Config types and JSON serialization
├── token.go           # JWT token generation and caching
├── client.go          # App Store Connect API client
├── certificates.go    # Certificate CRUD operations
├── bundleids.go       # Bundle ID and capabilities
├── devices.go         # Device registration and discovery
├── profiles.go        # Provisioning profile management
├── keychain.go        # macOS Keychain operations
├── setup.go           # Interactive setup wizard orchestration
└── status.go          # Status checking and validation
```

### Package Dependencies

```
signing/
├── config.go          → (no external deps)
├── token.go           → github.com/golang-jwt/jwt/v5
├── client.go          → net/http, token.go
├── certificates.go    → crypto/*, client.go
├── bundleids.go       → client.go
├── devices.go         → client.go, os/exec
├── profiles.go        → client.go
├── keychain.go        → os/exec
├── setup.go           → all of the above
└── status.go          → config.go, keychain.go
```

## End-to-End Flow Diagram

```
User runs: nanowave signing setup
                │
                ▼
    ┌───────────────────────┐
    │  Load or prompt for   │
    │  API Key credentials  │
    │  (Key ID, Issuer ID,  │
    │   .p8 path)           │
    └───────────┬───────────┘
                │
                ▼
    ┌───────────────────────┐
    │  Generate JWT token   │
    │  Verify API access    │
    └───────────┬───────────┘
                │
                ▼
    ┌───────────────────────┐
    │  Check for existing   │──── Found valid cert ───┐
    │  IOS_DEVELOPMENT cert │                          │
    └───────────┬───────────┘                          │
                │ Not found                            │
                ▼                                      │
    ┌───────────────────────┐                          │
    │  Generate CSR         │                          │
    │  Submit to Apple API  │                          │
    │  Get signed cert      │                          │
    │  Import to Keychain   │                          │
    └───────────┬───────────┘                          │
                │◄─────────────────────────────────────┘
                ▼
    ┌───────────────────────┐
    │  Detect connected     │
    │  iOS device UDID      │
    │  Register if new      │
    └───────────┬───────────┘
                │
                ▼
    ┌───────────────────────┐
    │  Register bundle ID   │──── Already exists ────┐
    │  (com.nanowave.X)     │                         │
    └───────────┬───────────┘                         │
                │◄────────────────────────────────────┘
                ▼
    ┌───────────────────────┐
    │  Enable capabilities  │
    │  from project_config  │
    └───────────┬───────────┘
                │
                ▼
    ┌───────────────────────┐
    │  Create provisioning  │
    │  profile (dev)        │
    │  Link: cert + bundle  │
    │        + device       │
    └───────────┬───────────┘
                │
                ▼
    ┌───────────────────────┐
    │  Install profile to   │
    │  ~/Library/Mobile...  │
    └───────────┬───────────┘
                │
                ▼
    ┌───────────────────────┐
    │  Save signing.json    │
    │  Update xcodegen.go   │
    │  config injection     │
    └───────────┬───────────┘
                │
                ▼
    ┌───────────────────────┐
    │  Done! Ready for      │
    │  device builds        │
    └───────────────────────┘
```

## Build Flow with Signing

```
nanowave build --device
        │
        ▼
  Load signing.json
        │
        ▼
  Check cert expiration ──── Expired? ──── Re-run setup
        │
        │ Valid
        ▼
  Check profile state ──── Invalid? ──── Regenerate profile
        │
        │ Active
        ▼
  Generate project.yml with Manual signing
        │
        ▼
  Run xcodegen generate
        │
        ▼
  Run xcodebuild with signing flags
        │
        ▼
  Install to connected device
```

## Security Considerations

### What Gets Stored

| Data | Where | Encryption |
|---|---|---|
| API Key credentials | `.nanowave/signing.json` | At rest (AES-256-GCM) |
| .p8 private key | User-specified path | File permissions only |
| Certificate private key | macOS Keychain | Keychain encryption |
| Signing certificate | macOS Keychain | Keychain encryption |
| Provisioning profile | `~/Library/MobileDevice/...` | None (standard Apple location) |

### Encryption Design for signing.json

The `.p8` path and sensitive IDs should be encrypted at rest:

```go
package signing

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
)

// EncryptedConfig wraps a Config with AES-256-GCM encryption.
type EncryptedConfig struct {
	Version int    `json:"version"`
	Nonce   []byte `json:"nonce"`
	Data    []byte `json:"data"` // Encrypted JSON
}

// deriveKey derives an encryption key from the machine's hardware UUID.
func deriveKey() ([]byte, error) {
	out, err := exec.Command("ioreg", "-rd1", "-c", "IOPlatformExpertDevice").Output()
	if err != nil {
		return nil, fmt.Errorf("failed to get hardware UUID: %w", err)
	}

	hash := sha256.Sum256(out)
	return hash[:], nil
}

// SaveEncrypted writes the config encrypted to disk.
func SaveEncrypted(config *Config, path string) error {
	key, err := deriveKey()
	if err != nil {
		return err
	}

	plaintext, err := json.Marshal(config)
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return fmt.Errorf("failed to create cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return fmt.Errorf("failed to create GCM: %w", err)
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return fmt.Errorf("failed to generate nonce: %w", err)
	}

	ciphertext := gcm.Seal(nil, nonce, plaintext, nil)

	enc := EncryptedConfig{
		Version: 1,
		Nonce:   nonce,
		Data:    ciphertext,
	}

	encJSON, err := json.Marshal(enc)
	if err != nil {
		return fmt.Errorf("failed to marshal encrypted config: %w", err)
	}

	return os.WriteFile(path, encJSON, 0600)
}

// LoadEncrypted reads and decrypts the config from disk.
func LoadEncrypted(path string) (*Config, error) {
	key, err := deriveKey()
	if err != nil {
		return nil, err
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config: %w", err)
	}

	var enc EncryptedConfig
	if err := json.Unmarshal(data, &enc); err != nil {
		return nil, fmt.Errorf("failed to parse encrypted config: %w", err)
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("failed to create cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("failed to create GCM: %w", err)
	}

	plaintext, err := gcm.Open(nil, enc.Nonce, enc.Data, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to decrypt config: %w", err)
	}

	var config Config
	if err := json.Unmarshal(plaintext, &config); err != nil {
		return nil, fmt.Errorf("failed to parse config: %w", err)
	}

	return &config, nil
}
```

### .gitignore Additions

```
# Signing
.nanowave/signing.json
*.p8
*.mobileprovision
*.pem
```

## Phase 1 vs Phase 2

### Phase 1: Device Builds (MVP)
- API key setup
- Development certificate
- Device registration (single connected device)
- Development provisioning profile
- Manual signing in xcodegen output
- `nanowave signing setup` / `nanowave signing status`

### Phase 2: Distribution
- Distribution certificate
- App Store provisioning profile
- IPA export with `xcodebuild -exportArchive`
- TestFlight upload via `altool` or Transporter API
- `nanowave distribute testflight` / `nanowave distribute appstore`
