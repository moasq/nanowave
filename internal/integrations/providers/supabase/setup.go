package supabase

import (
	"context"

	"github.com/moasq/nanowave/internal/config"
	"github.com/moasq/nanowave/internal/integrations"
)

// Setup delegates to the existing integrations.SetupSupabase/SetupSupabaseManual functions.
func (s *supabaseProvider) Setup(_ context.Context, req integrations.SetupRequest) error {
	if req.Manual {
		return integrations.SetupSupabaseManual(req.Store, req.AppName, req.ReadLineFn, req.PrintFn)
	}
	return integrations.SetupSupabase(req.Store, req.AppName, req.PrintFn, req.PickFn)
}

// Remove delegates to the existing integrations.RevokeSupabase function.
func (s *supabaseProvider) Remove(_ context.Context, store *integrations.IntegrationStore, appName string) error {
	return integrations.RevokeSupabase(store, appName)
}

// Status returns the current integration status for an app.
func (s *supabaseProvider) Status(_ context.Context, store *integrations.IntegrationStore, appName string) (integrations.ProviderStatus, error) {
	cfg, err := store.GetProvider(integrations.ProviderSupabase, appName)
	if err != nil {
		return integrations.ProviderStatus{}, err
	}
	if cfg == nil {
		return integrations.ProviderStatus{Configured: false}, nil
	}
	return integrations.ProviderStatus{
		Configured: true,
		ProjectURL: cfg.ProjectURL,
		HasAnonKey: cfg.AnonKey != "",
		HasPAT:     cfg.PAT != "",
	}, nil
}

// CLIAvailable checks if the Supabase CLI is installed.
func (s *supabaseProvider) CLIAvailable() bool {
	return config.CheckSupabaseCLI()
}
