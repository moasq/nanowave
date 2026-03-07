package asc

import "encoding/json"

// Result holds the output of an ASC operation.
type Result struct {
	Summary      string
	SessionID    string
	TotalCostUSD float64
	InputTokens  int
	OutputTokens int
	CacheRead    int
	CacheCreated int
	ToolsUsed    map[string]bool // tracks which tools the agent actually invoked
}

// PreflightResult holds context gathered during pre-flight checks.
type PreflightResult struct {
	AppID         string   // ASC app ID (empty if not matched)
	AppName       string   // display name
	BundleID      string   // bundle identifier
	ScreenshotDir string   // path to framed screenshots (empty if skipped)
	DeviceTypes   []string // e.g. ["IPHONE_67"]
	Localizations []string // from project_config.json
}

// Credential holds the App Store Connect API key credentials needed for xcodebuild authentication.
type Credential struct {
	KeyID      string `json:"key_id"`
	IssuerID   string `json:"issuer_id"`
	PrivateKey string `json:"private_key_pem"`
}

// Agreement is the typed JSON shape returned by `asc agreements list --output json`.
type Agreement struct {
	Status string `json:"status"`
	Type   string `json:"type"`
}

// Envelope is the App Store Connect API response envelope.
// `asc` CLI returns {"data": [...], "links": ..., "meta": ...}.
type Envelope struct {
	Data []struct {
		ID         string          `json:"id"`
		Attributes json.RawMessage `json:"attributes"`
	} `json:"data"`
}
