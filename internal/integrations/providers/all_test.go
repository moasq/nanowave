package providers

import (
	"testing"

	"github.com/moasq/nanowave/internal/integrations"
)

func TestRegisterAll(t *testing.T) {
	r := integrations.NewRegistry()
	RegisterAll(r)

	// Should have Supabase registered
	p, ok := r.Get(integrations.ProviderSupabase)
	if !ok {
		t.Fatal("expected Supabase to be registered")
	}
	if p.ID() != integrations.ProviderSupabase {
		t.Errorf("got ID %q, want %q", p.ID(), integrations.ProviderSupabase)
	}

	// Should have exactly 1 provider
	all := r.All()
	if len(all) != 1 {
		t.Errorf("expected 1 provider, got %d", len(all))
	}
}
