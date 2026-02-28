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

	// Should have RevenueCat registered
	p2, ok := r.Get(integrations.ProviderRevenueCat)
	if !ok {
		t.Fatal("expected RevenueCat to be registered")
	}
	if p2.ID() != integrations.ProviderRevenueCat {
		t.Errorf("got ID %q, want %q", p2.ID(), integrations.ProviderRevenueCat)
	}

	// Should have exactly 2 providers
	all := r.All()
	if len(all) != 2 {
		t.Errorf("expected 2 providers, got %d", len(all))
	}
}
