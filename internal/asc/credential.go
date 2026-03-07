package asc

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// LoadCredential reads the active ASC API key from the macOS keychain.
// The asc CLI stores credentials as JSON in generic password items with service "asc".
func LoadCredential() (*Credential, error) {
	configPath := filepath.Join(os.Getenv("HOME"), ".asc", "config.json")
	var defaultName string
	if data, err := os.ReadFile(configPath); err == nil {
		var cfg struct {
			DefaultKeyName string `json:"default_key_name"`
		}
		if json.Unmarshal(data, &cfg) == nil && cfg.DefaultKeyName != "" {
			defaultName = cfg.DefaultKeyName
		}
	}
	if defaultName == "" {
		return nil, fmt.Errorf("no default ASC key name found in ~/.asc/config.json")
	}

	account := "asc:credential:" + defaultName
	cmd := exec.Command("security", "find-generic-password", "-s", "asc", "-a", account, "-w")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("ASC credential not found in keychain (account=%s): %s", account, strings.TrimSpace(string(output)))
	}

	var cred Credential
	if err := json.Unmarshal(bytes.TrimSpace(output), &cred); err != nil {
		return nil, fmt.Errorf("failed to parse ASC credential: %w", err)
	}
	if cred.KeyID == "" || cred.IssuerID == "" || cred.PrivateKey == "" {
		return nil, fmt.Errorf("incomplete ASC credential (keyID=%s, issuerID=%s)", cred.KeyID, cred.IssuerID)
	}

	log.Printf("[keychain] loaded ASC credential keyID=%s issuerID=%s", cred.KeyID, cred.IssuerID)
	return &cred, nil
}

// WriteKeyFile writes the private key PEM to a temp file for xcodebuild authentication.
// Returns the file path. Caller must os.Remove the file when done.
func WriteKeyFile(cred *Credential, projectDir string) (string, error) {
	keyDir := filepath.Join(projectDir, "build")
	os.MkdirAll(keyDir, 0o700)
	keyPath := filepath.Join(keyDir, fmt.Sprintf("AuthKey_%s.p8", cred.KeyID))
	if err := os.WriteFile(keyPath, []byte(cred.PrivateKey), 0o600); err != nil {
		return "", fmt.Errorf("failed to write API key file: %w", err)
	}
	return keyPath, nil
}
