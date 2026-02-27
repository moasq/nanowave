package integrations

import (
	"context"
	"testing"
)

// mockMCPProvider implements Provider + MCPCapable.
type mockMCPProvider struct {
	mockProvider
	agentTools []string
	mcpTools   []string
}

func (m *mockMCPProvider) MCPServer(_ context.Context, req MCPRequest) (*MCPServerConfig, error) {
	return &MCPServerConfig{
		Name:    string(m.id),
		Command: "test-cmd",
		Args:    []string{"mcp", string(m.id)},
		Env: map[string]string{
			"TOKEN": req.PAT,
		},
	}, nil
}

func (m *mockMCPProvider) MCPTools() []string   { return m.mcpTools }
func (m *mockMCPProvider) AgentTools() []string { return m.agentTools }

// mockSetupUI is a no-op SetupUI for testing.
type mockSetupUI struct {
	infos    []string
	warnings []string
}

func (u *mockSetupUI) PromptSetup(_ context.Context, _ SetupCapable, _ Provider, _ *IntegrationStore, _ string) *IntegrationConfig {
	return nil
}
func (u *mockSetupUI) ValidateExisting(_ context.Context, _ SetupCapable, _ Provider, _ *IntegrationStore, _ string, cfg *IntegrationConfig) *IntegrationConfig {
	return cfg
}
func (u *mockSetupUI) Info(msg string)    { u.infos = append(u.infos, msg) }
func (u *mockSetupUI) Warning(msg string) { u.warnings = append(u.warnings, msg) }

func TestManager_AgentTools(t *testing.T) {
	r := NewRegistry()
	r.Register(&mockMCPProvider{
		mockProvider: mockProvider{id: "provider-a"},
		agentTools:   []string{"tool_a_1", "tool_a_2"},
		mcpTools:     []string{"mcp_a_1"},
	})
	r.Register(&mockMCPProvider{
		mockProvider: mockProvider{id: "provider-b"},
		agentTools:   []string{"tool_b_1"},
		mcpTools:     []string{"mcp_b_1", "mcp_b_2"},
	})

	m := NewManager(r, nil)
	active := []ActiveProvider{
		{Provider: must(r.Get("provider-a")), Config: &IntegrationConfig{}},
		{Provider: must(r.Get("provider-b")), Config: &IntegrationConfig{}},
	}

	tools := m.AgentTools(active)
	if len(tools) != 3 {
		t.Fatalf("expected 3 agent tools, got %d: %v", len(tools), tools)
	}
	want := []string{"tool_a_1", "tool_a_2", "tool_b_1"}
	for i, tool := range tools {
		if tool != want[i] {
			t.Errorf("tool[%d] = %q, want %q", i, tool, want[i])
		}
	}
}

func TestManager_MCPToolAllowlist(t *testing.T) {
	r := NewRegistry()
	r.Register(&mockMCPProvider{
		mockProvider: mockProvider{id: "prov"},
		mcpTools:     []string{"mcp_tool_1", "mcp_tool_2"},
	})

	m := NewManager(r, nil)
	active := []ActiveProvider{
		{Provider: must(r.Get("prov")), Config: &IntegrationConfig{}},
	}

	tools := m.MCPToolAllowlist(active)
	if len(tools) != 2 {
		t.Fatalf("expected 2 MCP tools, got %d", len(tools))
	}
}

func TestManager_MCPConfigs(t *testing.T) {
	r := NewRegistry()
	r.Register(&mockMCPProvider{
		mockProvider: mockProvider{id: "supa"},
		mcpTools:     []string{"mcp_supa_exec"},
	})

	m := NewManager(r, nil)
	active := []ActiveProvider{
		{
			Provider: must(r.Get("supa")),
			Config:   &IntegrationConfig{PAT: "test-pat", ProjectRef: "test-ref"},
		},
	}

	configs, err := m.MCPConfigs(context.Background(), active)
	if err != nil {
		t.Fatalf("MCPConfigs error: %v", err)
	}
	if len(configs) != 1 {
		t.Fatalf("expected 1 config, got %d", len(configs))
	}
	if configs[0].Env["TOKEN"] != "test-pat" {
		t.Errorf("expected TOKEN=test-pat, got %q", configs[0].Env["TOKEN"])
	}
}

func TestManager_AgentTools_SkipsNonMCPProviders(t *testing.T) {
	r := NewRegistry()
	// Register a plain provider without MCPCapable
	r.Register(&mockProvider{id: "plain"})

	m := NewManager(r, nil)
	active := []ActiveProvider{
		{Provider: must(r.Get("plain")), Config: &IntegrationConfig{}},
	}

	tools := m.AgentTools(active)
	if len(tools) != 0 {
		t.Errorf("expected 0 tools for non-MCP provider, got %d", len(tools))
	}
}

func TestManager_Resolve_UnknownProvider(t *testing.T) {
	r := NewRegistry()
	m := NewManager(r, NewIntegrationStore(t.TempDir()))
	ui := &mockSetupUI{}

	active, err := m.Resolve(context.Background(), "test-app", []string{"nonexistent"}, ui)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(active) != 0 {
		t.Errorf("expected 0 active providers, got %d", len(active))
	}
	if len(ui.warnings) == 0 {
		t.Error("expected warning for unknown provider")
	}
}

func must(p Provider, ok bool) Provider {
	if !ok {
		panic("provider not found")
	}
	return p
}
