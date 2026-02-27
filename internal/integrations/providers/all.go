// Package providers registers all integration providers.
// Pattern: Terraform's provider.Resources() — explicit, traceable registration.
// NOT init() magic — at 3-5 providers, explicit is more traceable.
package providers

import (
	"github.com/moasq/nanowave/internal/integrations"
	"github.com/moasq/nanowave/internal/integrations/providers/supabase"
)

// RegisterAll registers all available providers with the registry.
// This is the single registration point — adding a new provider
// requires one line here and one new package.
func RegisterAll(r *integrations.Registry) {
	r.Register(supabase.New())
	// r.Register(revenuecat.New())      // future
	// r.Register(appstoreconnect.New())  // future
}
