package integrations

import (
	"os"
	"path/filepath"
	"testing"
)

// validPAT is a test token matching the Supabase PAT format: sbp_ + 40 hex chars.
const validPAT = "sbp_0123456789abcdef0123456789abcdef01234567"

// validPAT2 is a second distinct valid token for priority/overwrite tests.
const validPAT2 = "sbp_abcdef0123456789abcdef0123456789abcdef01"

// validOAuthPAT is a valid OAuth-style token: sbp_oauth_ + 40 hex chars.
const validOAuthPAT = "sbp_oauth_0123456789abcdef0123456789abcdef01234567"

// --- Token validation tests ---

func TestIsValidPAT(t *testing.T) {
	tests := []struct {
		token string
		want  bool
	}{
		{validPAT, true},
		{validOAuthPAT, true},
		{validPAT2, true},
		{"sbp_0123456789abcdef0123456789abcdef0123456", false},  // 39 chars (too short)
		{"sbp_0123456789abcdef0123456789abcdef012345678", false}, // 41 chars (too long)
		{"sbp_UPPERCASE0123456789abcdef01234567890123", false},   // uppercase hex
		{"", false},
		{"random-string", false},
		{"\x1b\x1b", false},      // escape chars (the bug that caused garbage saves)
		{"sbp_not-hex-chars!!!", false},
	}
	for _, tt := range tests {
		t.Run(tt.token, func(t *testing.T) {
			if got := isValidPAT(tt.token); got != tt.want {
				t.Errorf("isValidPAT(%q) = %v, want %v", tt.token, got, tt.want)
			}
		})
	}
}

// --- readSupabasePAT tests ---

func TestReadSupabasePAT_EnvVar(t *testing.T) {
	t.Setenv("SUPABASE_ACCESS_TOKEN", validPAT)
	if got := readSupabasePAT(); got != validPAT {
		t.Errorf("readSupabasePAT() = %q, want %q", got, validPAT)
	}
}

func TestReadSupabasePAT_EnvVarTrimsWhitespace(t *testing.T) {
	t.Setenv("SUPABASE_ACCESS_TOKEN", "  "+validPAT+"  \n")
	if got := readSupabasePAT(); got != validPAT {
		t.Errorf("readSupabasePAT() = %q, want trimmed %q", got, validPAT)
	}
}

func TestReadSupabasePAT_EnvVarInvalidFormatRejected(t *testing.T) {
	t.Setenv("SUPABASE_ACCESS_TOKEN", "not-a-valid-token")
	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)

	if got := readSupabasePAT(); got != "" {
		t.Errorf("readSupabasePAT() = %q, want empty (invalid format)", got)
	}
}

func TestReadSupabasePAT_SupabaseConfigFile(t *testing.T) {
	t.Setenv("SUPABASE_ACCESS_TOKEN", "")
	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)

	dir := filepath.Join(tmpHome, ".supabase")
	os.MkdirAll(dir, 0700)
	os.WriteFile(filepath.Join(dir, "access-token"), []byte(validPAT), 0600)

	if got := readSupabasePAT(); got != validPAT {
		t.Errorf("readSupabasePAT() = %q, want %q", got, validPAT)
	}
}

func TestReadSupabasePAT_XDGConfigFile(t *testing.T) {
	t.Setenv("SUPABASE_ACCESS_TOKEN", "")
	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)

	dir := filepath.Join(tmpHome, ".config", "supabase")
	os.MkdirAll(dir, 0700)
	os.WriteFile(filepath.Join(dir, "access-token"), []byte(validPAT), 0600)

	if got := readSupabasePAT(); got != validPAT {
		t.Errorf("readSupabasePAT() = %q, want %q", got, validPAT)
	}
}

func TestReadSupabasePAT_FilePriority(t *testing.T) {
	t.Setenv("SUPABASE_ACCESS_TOKEN", "")
	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)

	// Write different valid tokens at each file location
	paths := map[string]string{
		filepath.Join(tmpHome, ".supabase", "access-token"):               validPAT,
		filepath.Join(tmpHome, ".config", "supabase", "access-token"):     validPAT2,
	}
	for p, tok := range paths {
		os.MkdirAll(filepath.Dir(p), 0700)
		os.WriteFile(p, []byte(tok), 0600)
	}

	// .supabase should win over .config/supabase
	if got := readSupabasePAT(); got != validPAT {
		t.Errorf("expected .supabase to win, got %q", got)
	}

	// Remove .supabase — .config/supabase should win
	os.Remove(filepath.Join(tmpHome, ".supabase", "access-token"))
	if got := readSupabasePAT(); got != validPAT2 {
		t.Errorf("expected .config/supabase to win, got %q", got)
	}
}

func TestReadSupabasePAT_EnvVarWinsOverFiles(t *testing.T) {
	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)

	// Write a file-based token
	dir := filepath.Join(tmpHome, ".supabase")
	os.MkdirAll(dir, 0700)
	os.WriteFile(filepath.Join(dir, "access-token"), []byte(validPAT2), 0600)

	// Env var should win
	t.Setenv("SUPABASE_ACCESS_TOKEN", validPAT)
	if got := readSupabasePAT(); got != validPAT {
		t.Errorf("expected env var to win, got %q", got)
	}
}

func TestReadSupabasePAT_EmptyReturnsEmpty(t *testing.T) {
	t.Setenv("SUPABASE_ACCESS_TOKEN", "")
	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)

	if got := readSupabasePAT(); got != "" {
		t.Errorf("readSupabasePAT() = %q, want empty", got)
	}
}

func TestReadSupabasePAT_InvalidFileTokenSkipped(t *testing.T) {
	t.Setenv("SUPABASE_ACCESS_TOKEN", "")
	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)

	// Write invalid token in .supabase, valid in .config/supabase
	os.MkdirAll(filepath.Join(tmpHome, ".supabase"), 0700)
	os.WriteFile(filepath.Join(tmpHome, ".supabase", "access-token"), []byte("garbage-token"), 0600)

	os.MkdirAll(filepath.Join(tmpHome, ".config", "supabase"), 0700)
	os.WriteFile(filepath.Join(tmpHome, ".config", "supabase", "access-token"), []byte(validPAT), 0600)

	if got := readSupabasePAT(); got != validPAT {
		t.Errorf("expected invalid token to be skipped, got %q", got)
	}
}

func TestReadSupabasePAT_EmptyFileSkipped(t *testing.T) {
	t.Setenv("SUPABASE_ACCESS_TOKEN", "")
	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)

	// Empty file in .supabase, valid token in .config/supabase
	os.MkdirAll(filepath.Join(tmpHome, ".supabase"), 0700)
	os.WriteFile(filepath.Join(tmpHome, ".supabase", "access-token"), []byte("  \n"), 0600)

	os.MkdirAll(filepath.Join(tmpHome, ".config", "supabase"), 0700)
	os.WriteFile(filepath.Join(tmpHome, ".config", "supabase", "access-token"), []byte(validPAT), 0600)

	if got := readSupabasePAT(); got != validPAT {
		t.Errorf("expected empty file to be skipped, got %q", got)
	}
}

// --- readFromKeychain tests ---

func TestReadFromKeychain_GracefulFailure(t *testing.T) {
	// On Linux CI there's no keychain/D-Bus — should return empty, not panic.
	if got := readFromKeychain("Supabase CLI", "supabase"); got != "" {
		t.Errorf("readFromKeychain() = %q, want empty (no keychain on Linux CI)", got)
	}
}

func TestReadFromKeychain_AllAccountNames(t *testing.T) {
	for _, account := range []string{"supabase", "access-token"} {
		if got := readFromKeychain("Supabase CLI", account); got != "" {
			t.Errorf("readFromKeychain(%q, %q) = %q, want empty", "Supabase CLI", account, got)
		}
	}
}

func TestReadFromKeychain_NanowaveService(t *testing.T) {
	// Our own service entry should also fail gracefully
	if got := readFromKeychain("nanowave", "supabase-pat"); got != "" {
		t.Errorf("readFromKeychain(nanowave, supabase-pat) = %q, want empty", got)
	}
}

// --- saveSupabasePAT tests ---

// On Linux CI without D-Bus, saveSupabasePAT falls back to file.
func TestSaveSupabasePAT_FallsBackToFile(t *testing.T) {
	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)

	saveSupabasePAT(validPAT)

	// On Linux CI, keyring.Set fails so it falls back to file
	path := filepath.Join(tmpHome, ".nanowave", "supabase-pat")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("expected file fallback after keyring failure: %v", err)
	}
	if string(data) != validPAT {
		t.Errorf("saved PAT = %q, want %q", string(data), validPAT)
	}

	// Verify file permissions are restricted
	info, _ := os.Stat(path)
	if perm := info.Mode().Perm(); perm != 0600 {
		t.Errorf("PAT file permissions = %o, want 0600", perm)
	}
}

func TestSaveSupabasePAT_OverwriteExisting(t *testing.T) {
	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)

	saveSupabasePAT(validPAT)
	saveSupabasePAT(validPAT2)

	path := filepath.Join(tmpHome, ".nanowave", "supabase-pat")
	data, _ := os.ReadFile(path)
	if string(data) != validPAT2 {
		t.Errorf("saved PAT = %q, want %q", string(data), validPAT2)
	}
}

// --- File-based reads now trigger caching ---

func TestFileReadCachesToKeyringFallback(t *testing.T) {
	t.Setenv("SUPABASE_ACCESS_TOKEN", "")
	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)

	// Write valid token in .supabase (file fallback path)
	dir := filepath.Join(tmpHome, ".supabase")
	os.MkdirAll(dir, 0700)
	os.WriteFile(filepath.Join(dir, "access-token"), []byte(validPAT), 0600)

	got := readSupabasePAT()
	if got != validPAT {
		t.Fatalf("readSupabasePAT() = %q, want %q", got, validPAT)
	}

	// On Linux CI, saveSupabasePAT falls back to file — verify it was cached
	cachePath := filepath.Join(tmpHome, ".nanowave", "supabase-pat")
	data, err := os.ReadFile(cachePath)
	if err != nil {
		t.Fatalf("expected cache file after file-based read: %v", err)
	}
	if string(data) != validPAT {
		t.Errorf("cached PAT = %q, want %q", string(data), validPAT)
	}
}

// --- Utility tests ---

func TestSanitizeProjectName(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"", "my-app"},
		{"MyApp", "my-app"},
		{"todoTracker", "todo-tracker"},
		{"hello world", "hello-world"},
		{"ALLCAPS", "a-l-l-c-a-p-s"},
		{"with_underscores", "with-underscores"},
		{"special!@#chars", "specialchars"},
		{"---dashes---", "dashes"},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			if got := sanitizeProjectName(tt.input); got != tt.want {
				t.Errorf("sanitizeProjectName(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestGeneratePassword(t *testing.T) {
	p1 := generatePassword()
	p2 := generatePassword()
	if len(p1) != 24 {
		t.Errorf("password length = %d, want 24", len(p1))
	}
	if p1 == p2 {
		t.Error("two generated passwords should not be identical")
	}
}
