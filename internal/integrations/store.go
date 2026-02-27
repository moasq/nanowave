package integrations

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/moasq/nanowave/internal/integrations/secrets"
)

// storeFile is the filename for persisted integration configs.
const storeFile = "integrations.json"

// defaultAppKey is the key used when migrating old flat-format configs.
const defaultAppKey = "_default"

// DefaultAppKey returns the key used for migrated legacy configs.
func DefaultAppKey() string { return defaultAppKey }

// storeData is the on-disk structure.
// Providers maps ProviderID → app name → config.
type storeData struct {
	Providers map[ProviderID]map[string]*IntegrationConfig `json:"providers"`
}

// secretRefPrefix marks a PAT field as a reference to a secret store key.
const secretRefPrefix = "secret:"

// IntegrationStore persists integration configs to ~/.nanowave/integrations.json.
// Sensitive fields (PAT) are stored in the OS keychain (or file fallback) and
// replaced with "secret:<key>" references in the JSON file.
type IntegrationStore struct {
	mu      sync.Mutex
	dir     string // directory containing the store file (e.g. ~/.nanowave)
	data    *storeData
	secrets secrets.SecretStore
}

// NewIntegrationStore creates a store rooted at the given directory.
// It initializes the best available secret store (OS keychain or file fallback).
func NewIntegrationStore(nanowaveRoot string) *IntegrationStore {
	return &IntegrationStore{
		dir: nanowaveRoot,
		data: &storeData{
			Providers: make(map[ProviderID]map[string]*IntegrationConfig),
		},
		secrets: secrets.New(nanowaveRoot),
	}
}

// NewIntegrationStoreWithSecrets creates a store with a custom secret store (for testing).
func NewIntegrationStoreWithSecrets(nanowaveRoot string, ss secrets.SecretStore) *IntegrationStore {
	return &IntegrationStore{
		dir: nanowaveRoot,
		data: &storeData{
			Providers: make(map[ProviderID]map[string]*IntegrationConfig),
		},
		secrets: ss,
	}
}

// Load reads the store from disk. Missing file is not an error.
// Automatically migrates old flat format (provider → config) to new per-app format.
func (s *IntegrationStore) Load() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	path := filepath.Join(s.dir, storeFile)
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("read integrations store: %w", err)
	}

	// Try new format first
	var sd storeData
	if err := json.Unmarshal(data, &sd); err != nil {
		return fmt.Errorf("parse integrations store: %w", err)
	}
	if sd.Providers == nil {
		sd.Providers = make(map[ProviderID]map[string]*IntegrationConfig)
	}

	// Detect old flat format: if any value under a provider key is an IntegrationConfig
	// (has "provider" field directly) rather than a map of configs, migrate it.
	migrated := false
	if needsMigration(data) {
		oldSD := struct {
			Providers map[ProviderID]*IntegrationConfig `json:"providers"`
		}{}
		if err := json.Unmarshal(data, &oldSD); err == nil && oldSD.Providers != nil {
			sd.Providers = make(map[ProviderID]map[string]*IntegrationConfig)
			for id, cfg := range oldSD.Providers {
				if cfg != nil {
					sd.Providers[id] = map[string]*IntegrationConfig{
						defaultAppKey: cfg,
					}
				}
			}
			migrated = true
		}
	}

	s.data = &sd

	// Persist migration if we converted the format
	if migrated {
		_ = s.saveLocked()
	}

	// Migrate raw PATs to secret store
	s.migratePATsToSecretStore()

	return nil
}

// migratePATsToSecretStore moves any raw PAT values from the JSON config
// into the secret store, replacing them with "secret:<key>" references.
// This is a one-time migration that happens on Load().
func (s *IntegrationStore) migratePATsToSecretStore() {
	needsSave := false
	for provID, apps := range s.data.Providers {
		for appName, cfg := range apps {
			if cfg.PAT != "" && !strings.HasPrefix(cfg.PAT, secretRefPrefix) {
				// Raw PAT found — migrate to secret store
				key := secrets.SecretKey(string(provID), appName, "pat")
				if err := s.secrets.Set(key, cfg.PAT); err != nil {
					// Migration is best-effort, but warn so the user knows
					fmt.Fprintf(os.Stderr, "warning: failed to migrate %s/%s credentials to secure storage: %v\n", provID, appName, err)
					continue
				}
				cfg.PAT = secretRefPrefix + key
				needsSave = true
			}
		}
	}
	if needsSave {
		_ = s.saveLocked()
	}
}

// needsMigration checks if the JSON has old flat format where provider values
// are IntegrationConfig objects (have "provider" field) instead of app-name maps.
func needsMigration(data []byte) bool {
	var raw struct {
		Providers map[string]json.RawMessage `json:"providers"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return false
	}
	for _, v := range raw.Providers {
		// Try to detect: if the value has a "provider" key directly, it's old format.
		// New format would have app-name keys mapping to objects.
		var obj map[string]json.RawMessage
		if err := json.Unmarshal(v, &obj); err != nil {
			continue
		}
		if _, hasProvider := obj["provider"]; hasProvider {
			return true
		}
	}
	return false
}

// saveLocked writes the current state to disk. Caller must already hold mu.
func (s *IntegrationStore) saveLocked() error {
	if err := os.MkdirAll(s.dir, 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(s.data, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(s.dir, storeFile), data, 0o600)
}

// GetProvider returns the config for a provider and app name, or nil if not configured.
// Secret references in the PAT field are resolved from the secret store transparently.
func (s *IntegrationStore) GetProvider(id ProviderID, appName string) (*IntegrationConfig, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	apps, ok := s.data.Providers[id]
	if !ok {
		return nil, nil
	}

	cfg, ok := apps[appName]
	if !ok {
		return nil, nil
	}
	cp := *cfg

	// Resolve secret reference
	if strings.HasPrefix(cp.PAT, secretRefPrefix) {
		key := strings.TrimPrefix(cp.PAT, secretRefPrefix)
		val, err := s.secrets.Get(key)
		if err == nil {
			cp.PAT = val
		} else {
			// Secret not found — clear PAT so callers know it's missing
			cp.PAT = ""
		}
	}

	return &cp, nil
}

// SetProvider stores or updates a provider config for a specific app.
// The PAT is automatically moved to the secret store and replaced with a reference.
func (s *IntegrationStore) SetProvider(cfg IntegrationConfig, appName string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Store PAT in secret store if it's a raw value (not already a reference)
	if cfg.PAT != "" && !strings.HasPrefix(cfg.PAT, secretRefPrefix) {
		key := secrets.SecretKey(string(cfg.Provider), appName, "pat")
		if err := s.secrets.Set(key, cfg.PAT); err != nil {
			return fmt.Errorf("failed to store credentials securely: %w", err)
		}
		cfg.PAT = secretRefPrefix + key
	}

	if s.data.Providers[cfg.Provider] == nil {
		s.data.Providers[cfg.Provider] = make(map[string]*IntegrationConfig)
	}
	s.data.Providers[cfg.Provider][appName] = &cfg
	return s.saveLocked()
}

// RemoveProvider deletes a provider config for a specific app.
// Also removes the corresponding secret from the secret store.
func (s *IntegrationStore) RemoveProvider(id ProviderID, appName string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Clean up secret if present
	if apps, ok := s.data.Providers[id]; ok {
		if cfg, ok := apps[appName]; ok && strings.HasPrefix(cfg.PAT, secretRefPrefix) {
			key := strings.TrimPrefix(cfg.PAT, secretRefPrefix)
			_ = s.secrets.Delete(key)
		}
	}

	apps, ok := s.data.Providers[id]
	if !ok {
		return s.saveLocked()
	}
	delete(apps, appName)
	// Clean up empty provider map
	if len(apps) == 0 {
		delete(s.data.Providers, id)
	}
	return s.saveLocked()
}

// AllAppNames returns all configured app names for a given provider.
func (s *IntegrationStore) AllAppNames(id ProviderID) []string {
	s.mu.Lock()
	defer s.mu.Unlock()

	apps, ok := s.data.Providers[id]
	if !ok {
		return nil
	}
	names := make([]string, 0, len(apps))
	for name := range apps {
		names = append(names, name)
	}
	return names
}

// AllStatuses returns the status of every known provider.
// Shows per-app statuses for providers with configured apps.
func (s *IntegrationStore) AllStatuses() []IntegrationStatus {
	s.mu.Lock()
	defer s.mu.Unlock()

	var statuses []IntegrationStatus
	for _, integ := range AllIntegrations() {
		apps, ok := s.data.Providers[integ.ID]
		if !ok || len(apps) == 0 {
			statuses = append(statuses, IntegrationStatus{
				Provider: integ.ID,
			})
			continue
		}

		for appName, cfg := range apps {
			statuses = append(statuses, IntegrationStatus{
				Provider:    integ.ID,
				AppName:     appName,
				Configured:  true,
				ProjectURL:  cfg.ProjectURL,
				HasAnonKey:  cfg.AnonKey != "",
				HasPAT:      cfg.PAT != "",
				ValidatedAt: time.Now().Format(time.RFC3339),
			})
		}
	}
	return statuses
}
