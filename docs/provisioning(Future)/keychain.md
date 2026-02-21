# macOS Keychain Operations

The macOS Keychain stores signing certificates and private keys. For local development, the default login keychain works fine. For CI/CD, you need temporary keychains to avoid polluting the system keychain.

## Keychain Basics

macOS has several keychains:
- **login.keychain-db** — User's default keychain (unlocked at login)
- **System.keychain** — System-wide, requires admin privileges
- **Custom keychains** — Created for CI/CD isolation

## Local Development Keychain

For local development, import certificates into the default login keychain:

```go
package signing

import (
	"fmt"
	"os/exec"
	"strings"
)

// DefaultKeychainPath returns the path to the user's login keychain.
func DefaultKeychainPath() (string, error) {
	out, err := exec.Command("security", "default-keychain", "-d", "user").Output()
	if err != nil {
		return "", fmt.Errorf("failed to get default keychain: %w", err)
	}

	// Output is like: "    \"/Users/foo/Library/Keychains/login.keychain-db\""
	path := strings.TrimSpace(string(out))
	path = strings.Trim(path, "\"")
	return path, nil
}

// ImportToDefaultKeychain imports a certificate and private key into the login keychain.
func ImportToDefaultKeychain(certPath, privateKeyPath string) error {
	keychainPath, err := DefaultKeychainPath()
	if err != nil {
		return err
	}

	return ImportToKeychain(certPath, privateKeyPath, keychainPath)
}
```

## CI/CD Temporary Keychain

On CI/CD systems, create an isolated keychain to avoid conflicts:

```go
// CIKeychain manages a temporary keychain for CI/CD builds.
type CIKeychain struct {
	Path     string
	Password string
}

// CreateCIKeychain creates a new temporary keychain for CI/CD use.
func CreateCIKeychain(name, password string) (*CIKeychain, error) {
	path := fmt.Sprintf("/tmp/%s.keychain-db", name)

	// Create the keychain
	if err := exec.Command("security", "create-keychain",
		"-p", password, path,
	).Run(); err != nil {
		return nil, fmt.Errorf("failed to create keychain: %w", err)
	}

	// Set keychain settings (no auto-lock)
	if err := exec.Command("security", "set-keychain-settings",
		"-lut", "21600", // 6 hours timeout
		path,
	).Run(); err != nil {
		return nil, fmt.Errorf("failed to set keychain settings: %w", err)
	}

	// Unlock the keychain
	if err := exec.Command("security", "unlock-keychain",
		"-p", password, path,
	).Run(); err != nil {
		return nil, fmt.Errorf("failed to unlock keychain: %w", err)
	}

	return &CIKeychain{Path: path, Password: password}, nil
}

// AddToSearchList adds this keychain to the search list so Xcode can find it.
func (k *CIKeychain) AddToSearchList() error {
	// Get current search list
	out, err := exec.Command("security", "list-keychains", "-d", "user").Output()
	if err != nil {
		return fmt.Errorf("failed to list keychains: %w", err)
	}

	// Parse existing keychains
	var keychains []string
	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(line)
		line = strings.Trim(line, "\"")
		if line != "" {
			keychains = append(keychains, line)
		}
	}

	// Add our keychain to the front of the list
	keychains = append([]string{k.Path}, keychains...)

	// Set the new search list
	args := append([]string{"list-keychains", "-d", "user", "-s"}, keychains...)
	if err := exec.Command("security", args...).Run(); err != nil {
		return fmt.Errorf("failed to set keychain search list: %w", err)
	}

	return nil
}

// Cleanup removes the temporary keychain.
func (k *CIKeychain) Cleanup() error {
	return exec.Command("security", "delete-keychain", k.Path).Run()
}
```

## Importing Certificates and Private Keys

```go
// ImportToKeychain imports both a certificate (.cer) and private key (.pem) into a keychain.
func ImportToKeychain(certPath, privateKeyPath, keychainPath string) error {
	// Import the private key
	if err := exec.Command("security", "import",
		privateKeyPath,
		"-k", keychainPath,
		"-T", "/usr/bin/codesign",
		"-T", "/usr/bin/security",
	).Run(); err != nil {
		return fmt.Errorf("failed to import private key: %w", err)
	}

	// Import the certificate
	if err := exec.Command("security", "import",
		certPath,
		"-k", keychainPath,
		"-T", "/usr/bin/codesign",
		"-T", "/usr/bin/security",
	).Run(); err != nil {
		return fmt.Errorf("failed to import certificate: %w", err)
	}

	return nil
}
```

## Setting Key Partition Lists

After importing, you must set the partition list to allow `codesign` to access the private key without a UI prompt. This is critical for CI/CD and automated builds.

```go
// SetKeyPartitionList allows codesign to access the keychain without prompting.
// This is REQUIRED for non-interactive (CI/CD) builds.
func SetKeyPartitionList(keychainPath, keychainPassword string) error {
	if err := exec.Command("security", "set-key-partition-list",
		"-S", "apple-tool:,apple:,codesign:",
		"-s",
		"-k", keychainPassword,
		keychainPath,
	).Run(); err != nil {
		return fmt.Errorf("failed to set key partition list: %w", err)
	}

	return nil
}
```

> **Why is this needed?** macOS requires explicit permission for processes to access keychain items. Without setting the partition list, `codesign` will either fail silently or show a UI dialog asking for permission — which blocks CI/CD pipelines.

## Querying Installed Identities

Verify that certificates are properly installed and ready for signing:

```go
// SigningIdentity holds info about a code signing identity in the keychain.
type SigningIdentity struct {
	Hash    string // SHA-1 hash of the certificate
	Name    string // e.g., "Apple Development: John Doe (TEAMID)"
	IsValid bool
}

// FindSigningIdentities returns all valid signing identities in the keychain.
func FindSigningIdentities(keychainPath string) ([]SigningIdentity, error) {
	args := []string{"find-identity", "-v", "-p", "codesigning"}
	if keychainPath != "" {
		args = append(args, keychainPath)
	}

	out, err := exec.Command("security", args...).Output()
	if err != nil {
		return nil, fmt.Errorf("failed to find identities: %w", err)
	}

	var identities []SigningIdentity
	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "Policy:") || strings.Contains(line, "valid identities found") {
			continue
		}

		// Format: "  1) HASH \"Name\""
		parts := strings.SplitN(line, ") ", 2)
		if len(parts) != 2 {
			continue
		}

		hashAndName := strings.TrimSpace(parts[1])
		spaceIdx := strings.Index(hashAndName, " ")
		if spaceIdx == -1 {
			continue
		}

		hash := hashAndName[:spaceIdx]
		name := strings.Trim(hashAndName[spaceIdx+1:], "\" ")

		identities = append(identities, SigningIdentity{
			Hash:    hash,
			Name:    name,
			IsValid: true,
		})
	}

	return identities, nil
}
```

### Example Output

```
$ security find-identity -v -p codesigning
  1) ABC123DEF456... "Apple Development: John Doe (TEAMID)"
  2) 789GHI012JKL... "Apple Distribution: John Doe (TEAMID)"
     2 valid identities found
```

## Full CI/CD Keychain Flow

Here's the complete flow for setting up signing on a CI/CD machine:

```go
// SetupCISigning sets up a complete signing environment for CI/CD.
func SetupCISigning(certPath, privateKeyPath, profilePath, keychainPassword string) (*CIKeychain, error) {
	// 1. Create temporary keychain
	keychain, err := CreateCIKeychain("nanowave-build", keychainPassword)
	if err != nil {
		return nil, fmt.Errorf("failed to create keychain: %w", err)
	}

	// 2. Add to search list
	if err := keychain.AddToSearchList(); err != nil {
		keychain.Cleanup()
		return nil, fmt.Errorf("failed to add keychain to search list: %w", err)
	}

	// 3. Import certificate and private key
	if err := ImportToKeychain(certPath, privateKeyPath, keychain.Path); err != nil {
		keychain.Cleanup()
		return nil, fmt.Errorf("failed to import certificate: %w", err)
	}

	// 4. Set partition list for non-interactive access
	if err := SetKeyPartitionList(keychain.Path, keychainPassword); err != nil {
		keychain.Cleanup()
		return nil, fmt.Errorf("failed to set partition list: %w", err)
	}

	// 5. Verify identity is available
	identities, err := FindSigningIdentities(keychain.Path)
	if err != nil || len(identities) == 0 {
		keychain.Cleanup()
		return nil, fmt.Errorf("no signing identities found after import")
	}

	return keychain, nil
}
```

## Cleanup

Always clean up temporary keychains after a build:

```go
// CleanupCISigning removes the temporary keychain and restores the search list.
func CleanupCISigning(keychain *CIKeychain) error {
	return keychain.Cleanup()
}
```

## Troubleshooting

| Issue | Solution |
|---|---|
| `codesign` prompts for keychain password | Set key partition list with `SetKeyPartitionList` |
| "No identity found" during build | Check `security find-identity -v -p codesigning` |
| Keychain locked during CI build | Use `security unlock-keychain` before build |
| "User interaction is not allowed" | Keychain is locked or partition list not set |
| Certificate not trusted | Install Apple intermediate certificates: `security add-trusted-cert` |
