package asc

import "encoding/json"

// Version state constants (finite API-defined set)
const (
	VersionPrepareForSubmission    = "PREPARE_FOR_SUBMISSION"
	VersionWaitingForReview        = "WAITING_FOR_REVIEW"
	VersionInReview                = "IN_REVIEW"
	VersionDeveloperRejected       = "DEVELOPER_REJECTED"
	VersionRejected                = "REJECTED"
	VersionReadyForSale            = "READY_FOR_SALE"
	VersionPendingDeveloperRelease = "PENDING_DEVELOPER_RELEASE"
	VersionProcessingForAppStore   = "PROCESSING_FOR_APP_STORE"
)

// VersionInfo holds the ID, version string, and state for an App Store version.
type VersionInfo struct {
	ID            string
	VersionString string
	State         string
}

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

	// Agreement state
	AgreementsOK bool
	Agreements   []Agreement

	// Version state
	VersionID     string // version in editable state (empty if none)
	VersionString string // e.g. "1.0.0"
	VersionState  string // e.g. "PREPARE_FOR_SUBMISSION"
	AllVersions   []VersionInfo // every version returned by API

	// Build state
	LatestBuildID      string // most recent build ID
	LatestBuildVersion string // e.g. "42"
	BuildState         string // e.g. "VALID", "PROCESSING"

	// Flags
	IconReady bool
	HasAPIKey bool

	// Submission readiness flags
	HasSignIn    bool // project contains authentication/login code
	CollectsData bool // project appears to collect user data (analytics, APIs, etc.)
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
