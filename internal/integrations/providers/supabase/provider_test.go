package supabase

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/moasq/nanowave/internal/integrations"
)

func TestProvider_ID(t *testing.T) {
	p := New()
	if p.ID() != integrations.ProviderSupabase {
		t.Errorf("got ID %q, want %q", p.ID(), integrations.ProviderSupabase)
	}
}

func TestProvider_Meta(t *testing.T) {
	p := New()
	meta := p.Meta()
	if meta.Name != "Supabase" {
		t.Errorf("got Name %q, want %q", meta.Name, "Supabase")
	}
	if meta.SPMPackage != "supabase-swift" {
		t.Errorf("got SPMPackage %q, want %q", meta.SPMPackage, "supabase-swift")
	}
}

func TestProvider_MCPTools(t *testing.T) {
	p := New()
	mc, ok := p.(integrations.MCPCapable)
	if !ok {
		t.Fatal("provider should implement MCPCapable")
	}
	tools := mc.MCPTools()
	if len(tools) == 0 {
		t.Fatal("expected non-empty MCP tools list")
	}
	// All tools should start with "mcp__supabase__"
	for _, tool := range tools {
		if !strings.HasPrefix(tool, "mcp__supabase__") {
			t.Errorf("tool %q doesn't have expected prefix", tool)
		}
	}
}

func TestProvider_AgentTools(t *testing.T) {
	p := New()
	mc, ok := p.(integrations.MCPCapable)
	if !ok {
		t.Fatal("provider should implement MCPCapable")
	}
	tools := mc.AgentTools()
	if len(tools) == 0 {
		t.Fatal("expected non-empty agent tools list")
	}
}

func TestProvider_MCPServer(t *testing.T) {
	p := New()
	mc := p.(integrations.MCPCapable)
	cfg, err := mc.MCPServer(context.Background(), integrations.MCPRequest{
		PAT:        "test-pat",
		ProjectRef: "test-ref",
	})
	if err != nil {
		t.Fatalf("MCPServer error: %v", err)
	}
	if cfg.Name != "supabase" {
		t.Errorf("got name %q, want %q", cfg.Name, "supabase")
	}
	if cfg.Env["SUPABASE_ACCESS_TOKEN"] != "test-pat" {
		t.Errorf("expected token in env")
	}
}

func TestProvider_PromptContribution(t *testing.T) {
	// Set up a temp store with config
	tmpDir := t.TempDir()
	nanowaveDir := filepath.Join(tmpDir, ".nanowave")
	os.MkdirAll(nanowaveDir, 0o755)

	storeData := map[string]any{
		"providers": map[string]any{
			"supabase": map[string]any{
				"TestApp": map[string]any{
					"provider":    "supabase",
					"project_url": "https://test.supabase.co",
					"project_ref": "test",
					"anon_key":    "test-key",
					"pat":         "test-pat",
				},
			},
		},
	}
	data, _ := json.MarshalIndent(storeData, "", "  ")
	os.WriteFile(filepath.Join(nanowaveDir, "integrations.json"), data, 0o644)

	store := integrations.NewIntegrationStore(nanowaveDir)
	if err := store.Load(); err != nil {
		t.Fatalf("store.Load: %v", err)
	}

	p := New()
	pc := p.(integrations.PromptCapable)

	contrib, err := pc.PromptContribution(context.Background(), integrations.PromptRequest{
		AppName: "TestApp",
		Models: []integrations.ModelRef{
			{
				Name: "Post",
				Properties: []integrations.PropertyRef{
					{Name: "id", Type: "UUID"},
					{Name: "title", Type: "String"},
				},
			},
		},
		AuthMethods: []string{"email"},
		Store:       store,
	})
	if err != nil {
		t.Fatalf("PromptContribution error: %v", err)
	}
	if !strings.Contains(contrib.SystemBlock, "<integration-config>") {
		t.Error("expected <integration-config> in system block")
	}
	if !strings.Contains(contrib.SystemBlock, "test-key") {
		t.Error("expected anon key in system block")
	}
	if !strings.Contains(contrib.SystemBlock, "posts") {
		t.Error("expected table name 'posts' in system block")
	}
}
