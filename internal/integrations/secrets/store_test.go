package secrets

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/zalando/go-keyring"
)

func init() {
	// Use mock keychain for all tests — CI-safe, no host keychain needed.
	keyring.MockInit()
}

func TestKeychainStore_CRUD(t *testing.T) {
	s := newKeychainStore()
	key := SecretKey("supabase", "TestApp", "pat")

	// Get non-existent
	_, err := s.Get(key)
	if err != ErrNotFound {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}

	// Set
	if err := s.Set(key, "sbp_test123"); err != nil {
		t.Fatalf("Set failed: %v", err)
	}

	// Get
	val, err := s.Get(key)
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if val != "sbp_test123" {
		t.Errorf("got %q, want %q", val, "sbp_test123")
	}

	// Delete
	if err := s.Delete(key); err != nil {
		t.Fatalf("Delete failed: %v", err)
	}

	// Get after delete
	_, err = s.Get(key)
	if err != ErrNotFound {
		t.Fatalf("expected ErrNotFound after delete, got %v", err)
	}

	// Delete non-existent should not error
	if err := s.Delete(key); err != nil {
		t.Fatalf("Delete of non-existent key should not error: %v", err)
	}
}

func TestFileStore_CRUD(t *testing.T) {
	dir := t.TempDir()
	s := newFileStore(dir)
	key := SecretKey("supabase", "TestApp", "pat")

	// Get non-existent
	_, err := s.Get(key)
	if err != ErrNotFound {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}

	// Set
	if err := s.Set(key, "sbp_file123"); err != nil {
		t.Fatalf("Set failed: %v", err)
	}

	// Verify file permissions
	info, err := os.Stat(filepath.Join(dir, secretsFile))
	if err != nil {
		t.Fatalf("stat secrets file: %v", err)
	}
	if perm := info.Mode().Perm(); perm != secretsFileMode {
		t.Errorf("file permissions: got %o, want %o", perm, secretsFileMode)
	}

	// Get
	val, err := s.Get(key)
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if val != "sbp_file123" {
		t.Errorf("got %q, want %q", val, "sbp_file123")
	}

	// Delete
	if err := s.Delete(key); err != nil {
		t.Fatalf("Delete failed: %v", err)
	}

	_, err = s.Get(key)
	if err != ErrNotFound {
		t.Fatalf("expected ErrNotFound after delete, got %v", err)
	}
}

func TestFileStore_Persistence(t *testing.T) {
	dir := t.TempDir()
	key := "test/key"

	// Write with one instance
	s1 := newFileStore(dir)
	if err := s1.Set(key, "persisted_value"); err != nil {
		t.Fatalf("Set failed: %v", err)
	}

	// Read with a new instance
	s2 := newFileStore(dir)
	val, err := s2.Get(key)
	if err != nil {
		t.Fatalf("Get on new instance failed: %v", err)
	}
	if val != "persisted_value" {
		t.Errorf("got %q, want %q", val, "persisted_value")
	}
}

func TestSecretKey(t *testing.T) {
	key := SecretKey("supabase", "MyApp", "pat")
	if key != "supabase/MyApp/pat" {
		t.Errorf("got %q, want %q", key, "supabase/MyApp/pat")
	}
}

func TestNew_FallsBackToFileStore(t *testing.T) {
	// With mock keychain, New should return keychain store (mock succeeds)
	dir := t.TempDir()
	s := New(dir)
	if s == nil {
		t.Fatal("New returned nil")
	}

	// Should be usable
	key := "test/probe"
	if err := s.Set(key, "val"); err != nil {
		t.Fatalf("Set failed: %v", err)
	}
	val, err := s.Get(key)
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if val != "val" {
		t.Errorf("got %q, want %q", val, "val")
	}
	_ = s.Delete(key)
}
