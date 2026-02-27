package orchestration

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/moasq/nanowave/internal/integrations"
)

// updateGolden controls whether golden files are rewritten.
// Run with: UPDATE_GOLDEN=1 go test -run Golden
var updateGolden = os.Getenv("UPDATE_GOLDEN") == "1"

func goldenPath(name string) string {
	return filepath.Join("testdata", name+".golden")
}

func assertGolden(t *testing.T, name, got string) {
	t.Helper()
	path := goldenPath(name)
	if updateGolden {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatalf("mkdir for golden: %v", err)
		}
		if err := os.WriteFile(path, []byte(got), 0o644); err != nil {
			t.Fatalf("write golden %s: %v", path, err)
		}
		return
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read golden %s: %v (run with -update-golden to create)", path, err)
	}
	if got != string(want) {
		t.Errorf("golden mismatch for %s:\n--- want ---\n%s\n--- got ---\n%s", name, string(want), got)
	}
}

// TestGolden_BaseAgenticTools locks the output of baseAgenticTools.
func TestGolden_BaseAgenticTools(t *testing.T) {
	tools := make([]string, len(baseAgenticTools))
	copy(tools, baseAgenticTools)
	got := strings.Join(tools, "\n") + "\n"
	assertGolden(t, "agentic_tools_none", got)
}

// TestGolden_WriteMCPConfig_NoIntegrations locks the MCP config with no integrations.
func TestGolden_WriteMCPConfig_NoIntegrations(t *testing.T) {
	dir := t.TempDir()
	if err := writeMCPConfig(dir, nil); err != nil {
		t.Fatalf("writeMCPConfig() error: %v", err)
	}
	data, err := os.ReadFile(filepath.Join(dir, ".mcp.json"))
	if err != nil {
		t.Fatalf("read .mcp.json: %v", err)
	}
	// Normalize to stable JSON
	got := normalizeJSON(t, data)
	assertGolden(t, "mcp_config_none", got)
}

// TestGolden_WriteMCPConfig_Supabase locks the MCP config with Supabase active.
func TestGolden_WriteMCPConfig_Supabase(t *testing.T) {
	dir := t.TempDir()
	configs := []integrations.MCPServerConfig{
		{
			Name:    "supabase",
			Command: "nanowave",
			Args:    []string{"mcp", "supabase"},
			Env: map[string]string{
				"SUPABASE_ACCESS_TOKEN": "sbp_test_token_123",
				"SUPABASE_PROJECT_REF":  "abcdefghijkl",
			},
		},
	}
	if err := writeMCPConfig(dir, configs); err != nil {
		t.Fatalf("writeMCPConfig() error: %v", err)
	}
	data, err := os.ReadFile(filepath.Join(dir, ".mcp.json"))
	if err != nil {
		t.Fatalf("read .mcp.json: %v", err)
	}
	got := normalizeJSON(t, data)
	assertGolden(t, "mcp_config_supabase", got)
}

// TestGolden_WriteSettingsShared_NoIntegrations locks settings output with no integrations.
func TestGolden_WriteSettingsShared_NoIntegrations(t *testing.T) {
	dir := t.TempDir()
	claudeDir := filepath.Join(dir, ".claude")
	if err := os.MkdirAll(claudeDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := writeSettingsShared(dir, nil); err != nil {
		t.Fatalf("writeSettingsShared() error: %v", err)
	}
	data, err := os.ReadFile(filepath.Join(claudeDir, "settings.json"))
	if err != nil {
		t.Fatalf("read settings.json: %v", err)
	}
	assertGolden(t, "settings_shared_none", string(data))
}

// TestGolden_WriteSettingsShared_Supabase locks settings output with Supabase MCP tools.
func TestGolden_WriteSettingsShared_Supabase(t *testing.T) {
	dir := t.TempDir()
	claudeDir := filepath.Join(dir, ".claude")
	if err := os.MkdirAll(claudeDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	supabaseTools := []string{
		"mcp__supabase__execute_sql",
		"mcp__supabase__list_tables",
		"mcp__supabase__apply_migration",
		"mcp__supabase__list_storage_buckets",
		"mcp__supabase__get_project_url",
		"mcp__supabase__get_anon_key",
		"mcp__supabase__get_logs",
		"mcp__supabase__configure_auth_providers",
		"mcp__supabase__get_auth_config",
	}
	if err := writeSettingsShared(dir, supabaseTools); err != nil {
		t.Fatalf("writeSettingsShared() error: %v", err)
	}
	data, err := os.ReadFile(filepath.Join(claudeDir, "settings.json"))
	if err != nil {
		t.Fatalf("read settings.json: %v", err)
	}
	assertGolden(t, "settings_shared_supabase", string(data))
}

// normalizeJSON re-marshals JSON to ensure stable key ordering.
func normalizeJSON(t *testing.T, data []byte) string {
	t.Helper()
	var v any
	if err := json.Unmarshal(data, &v); err != nil {
		t.Fatalf("unmarshal JSON: %v", err)
	}
	out, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		t.Fatalf("marshal JSON: %v", err)
	}
	return string(out) + "\n"
}
