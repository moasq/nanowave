package secrets

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
)

const secretsFile = "secrets.json"
const secretsFileMode = 0o600

// fileStore is a file-based fallback for environments without an OS keychain.
// Secrets are stored in a JSON file with 0600 permissions.
type fileStore struct {
	mu   sync.Mutex
	path string
}

func newFileStore(dir string) *fileStore {
	return &fileStore{path: filepath.Join(dir, secretsFile)}
}

func (f *fileStore) Get(key string) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	data, err := f.load()
	if err != nil {
		return "", err
	}
	val, ok := data[key]
	if !ok {
		return "", ErrNotFound
	}
	return val, nil
}

func (f *fileStore) Set(key, value string) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	data, err := f.load()
	if err != nil {
		return err
	}
	data[key] = value
	return f.save(data)
}

func (f *fileStore) Delete(key string) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	data, err := f.load()
	if err != nil {
		return err
	}
	delete(data, key)
	return f.save(data)
}

func (f *fileStore) load() (map[string]string, error) {
	raw, err := os.ReadFile(f.path)
	if os.IsNotExist(err) {
		return make(map[string]string), nil
	}
	if err != nil {
		return nil, err
	}
	var data map[string]string
	if err := json.Unmarshal(raw, &data); err != nil {
		return make(map[string]string), nil // corrupt file, start fresh
	}
	return data, nil
}

func (f *fileStore) save(data map[string]string) error {
	dir := filepath.Dir(f.path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	raw, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(f.path, raw, secretsFileMode)
}
