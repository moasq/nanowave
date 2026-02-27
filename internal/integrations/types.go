package integrations

// ProviderID identifies a backend integration provider.
type ProviderID string

const (
	ProviderSupabase ProviderID = "supabase"
)

// IntegrationConfig stores credentials and connection details for a backend provider.
type IntegrationConfig struct {
	Provider   ProviderID `json:"provider"`
	ProjectURL string     `json:"project_url"`  // https://xyz.supabase.co
	ProjectRef string     `json:"project_ref"`  // xyz (extracted from URL)
	AnonKey    string     `json:"anon_key"`     // public anon key
	PAT        string     `json:"pat,omitempty"` // Personal Access Token for MCP
}

// IntegrationStatus summarizes the configuration state of a provider for a specific app.
type IntegrationStatus struct {
	Provider    ProviderID `json:"provider"`
	AppName     string     `json:"app_name,omitempty"`
	Configured  bool       `json:"configured"`
	ProjectURL  string     `json:"project_url,omitempty"`
	HasAnonKey  bool       `json:"has_anon_key"`
	HasPAT      bool       `json:"has_pat"`
	ValidatedAt string     `json:"validated_at,omitempty"`
}

// BackendNeeds indicates which backend capabilities an app requires.
type BackendNeeds struct {
	Auth        bool     `json:"auth"`
	AuthMethods []string `json:"auth_methods,omitempty"` // "email", "apple", "google", "anonymous"
	DB          bool     `json:"db"`
	Storage     bool     `json:"storage"`
}

// NeedsBackend returns true if any backend capability is required.
func (b *BackendNeeds) NeedsBackend() bool {
	return b != nil && (b.Auth || b.DB || b.Storage)
}

