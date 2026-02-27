package integrations

import (
	"testing"
)

// mockProvider is a minimal Provider for testing.
type mockProvider struct {
	id   ProviderID
	meta ProviderMeta
}

func (m *mockProvider) ID() ProviderID    { return m.id }
func (m *mockProvider) Meta() ProviderMeta { return m.meta }

func TestRegistry_RegisterAndGet(t *testing.T) {
	r := NewRegistry()
	p := &mockProvider{id: "test-provider", meta: ProviderMeta{Name: "Test"}}

	r.Register(p)

	got, ok := r.Get("test-provider")
	if !ok {
		t.Fatal("expected provider to be found")
	}
	if got.ID() != "test-provider" {
		t.Errorf("got ID %q, want %q", got.ID(), "test-provider")
	}
}

func TestRegistry_GetMissing(t *testing.T) {
	r := NewRegistry()

	_, ok := r.Get("nonexistent")
	if ok {
		t.Fatal("expected provider not to be found")
	}
}

func TestRegistry_DuplicatePanics(t *testing.T) {
	r := NewRegistry()
	p := &mockProvider{id: "dup", meta: ProviderMeta{Name: "Dup"}}
	r.Register(p)

	defer func() {
		if recover() == nil {
			t.Fatal("expected panic on duplicate registration")
		}
	}()
	r.Register(p) // should panic
}

func TestRegistry_All_StableOrder(t *testing.T) {
	r := NewRegistry()
	r.Register(&mockProvider{id: "bravo", meta: ProviderMeta{Name: "Bravo"}})
	r.Register(&mockProvider{id: "alpha", meta: ProviderMeta{Name: "Alpha"}})
	r.Register(&mockProvider{id: "charlie", meta: ProviderMeta{Name: "Charlie"}})

	all := r.All()
	if len(all) != 3 {
		t.Fatalf("expected 3 providers, got %d", len(all))
	}

	// Should be alphabetically sorted by ID
	expected := []ProviderID{"alpha", "bravo", "charlie"}
	for i, p := range all {
		if p.ID() != expected[i] {
			t.Errorf("position %d: got %q, want %q", i, p.ID(), expected[i])
		}
	}
}
