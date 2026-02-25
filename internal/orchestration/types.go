package orchestration

import (
	"encoding/json"
	"fmt"
	"strings"
)

// BuildResult is the final output of a successful Build pipeline.
type BuildResult struct {
	AppName           string
	Description       string
	ProjectDir        string
	BundleID          string
	DeviceFamily      string
	Platform          string
	Platforms         []string
	WatchProjectShape string
	Features          []Feature
	FileCount         int
	PlannedFiles      int
	CompletedFiles    int
	CompletionPasses  int
	SessionID         string
	TotalCostUSD      float64
	InputTokens       int
	OutputTokens      int
	CacheRead         int
	CacheCreated      int
}

// IntentDecision is the parsed output from the pre-analysis intent router.
// Hints are advisory only; analyzer/planner may override based on explicit user intent.
type IntentDecision struct {
	Operation             string   `json:"operation"`
	PlatformHint          string   `json:"platform_hint"`
	PlatformHints         []string `json:"platform_hints"`
	DeviceFamilyHint      string   `json:"device_family_hint"`
	WatchProjectShapeHint string   `json:"watch_project_shape_hint"`
	Confidence            float64  `json:"confidence"`
	Reason                string   `json:"reason"`
	UsedLLM               bool     `json:"used_llm"`
}

// AnalysisResult is the parsed output from the analyzer phase.
type AnalysisResult struct {
	AppName     string    `json:"app_name"`
	Description string    `json:"description"`
	Features    []Feature `json:"features"`
	CoreFlow    string    `json:"core_flow"`
	Deferred    []string  `json:"deferred"`
}

// Feature is a single app feature from the analyzer.
type Feature struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

// PlannerResult is the parsed output from the planner phase.
type PlannerResult struct {
	Design            DesignSystem    `json:"design"`
	DeviceFamily      string          `json:"device_family"`
	Platform          string          `json:"platform"`
	Platforms         []string        `json:"platforms"`
	WatchProjectShape string          `json:"watch_project_shape"`
	Files             []FilePlan      `json:"files"`
	Models            []ModelPlan     `json:"models"`
	Permissions       []Permission    `json:"permissions"`
	Extensions        []ExtensionPlan `json:"extensions"`
	Localizations     []string        `json:"localizations"`
	RuleKeys          []string        `json:"rule_keys"`
	BuildOrder        []string        `json:"build_order"`
}

// GetDeviceFamily returns the device family, defaulting to "iphone" for iOS.
// Non-iOS platforms (macOS, tvOS, visionOS, watchOS) return "" when unset.
func (p *PlannerResult) GetDeviceFamily() string {
	if p == nil || p.DeviceFamily == "" {
		if p != nil && (IsMacOS(p.Platform) || IsTvOS(p.Platform) || IsVisionOS(p.Platform) || IsWatchOS(p.Platform)) {
			return ""
		}
		return "iphone"
	}
	return p.DeviceFamily
}

// GetPlatform returns the target platform, defaulting to "ios".
func (p *PlannerResult) GetPlatform() string {
	if p == nil || p.Platform == "" {
		return PlatformIOS
	}
	return p.Platform
}

// GetPlatforms returns the list of target platforms. Falls back to [Platform] if Platforms is empty.
func (p *PlannerResult) GetPlatforms() []string {
	if p == nil {
		return []string{PlatformIOS}
	}
	if len(p.Platforms) > 0 {
		return p.Platforms
	}
	return []string{p.GetPlatform()}
}

// IsMultiPlatform returns true when the plan targets more than one platform.
func (p *PlannerResult) IsMultiPlatform() bool {
	return len(p.GetPlatforms()) > 1
}

// GetWatchProjectShape returns the watch project shape, defaulting to "watch_only" when watchOS.
func (p *PlannerResult) GetWatchProjectShape() string {
	if p == nil || p.WatchProjectShape == "" {
		if p != nil && IsWatchOS(p.Platform) {
			return WatchShapeStandalone
		}
		// For multi-platform with watchOS, default to paired
		if p != nil && p.IsMultiPlatform() {
			for _, plat := range p.GetPlatforms() {
				if IsWatchOS(plat) {
					return WatchShapePaired
				}
			}
		}
		return ""
	}
	return p.WatchProjectShape
}

// HasRuleKey returns true if the plan includes the given rule key.
func (p *PlannerResult) HasRuleKey(key string) bool {
	if p == nil {
		return false
	}
	for _, k := range p.RuleKeys {
		if k == key {
			return true
		}
	}
	return false
}

// ExtensionPlan describes a secondary Xcode target (widget, live activity, etc.)
type ExtensionPlan struct {
	Kind         string            `json:"kind"`
	Name         string            `json:"name"`
	Purpose      string            `json:"purpose"`
	Platform     string            `json:"platform,omitempty"`
	InfoPlist    map[string]any    `json:"info_plist,omitempty"`
	Entitlements map[string]any    `json:"entitlements,omitempty"`
	Settings     map[string]string `json:"settings,omitempty"`
}

// DesignSystem holds the visual design specification.
type DesignSystem struct {
	Navigation   string  `json:"navigation"`
	Palette      Palette `json:"palette"`
	FontDesign   string  `json:"font_design"`
	CornerRadius int     `json:"corner_radius"`
	Density      string  `json:"density"`
	Surfaces     string  `json:"surfaces"`
	AppMood      string  `json:"app_mood"`
}

// Palette holds the 5-color hex palette.
type Palette struct {
	Primary    string `json:"primary"`
	Secondary  string `json:"secondary"`
	Accent     string `json:"accent"`
	Background string `json:"background"`
	Surface    string `json:"surface"`
}

// FilePlan describes a single file in the build plan.
type FilePlan struct {
	Path       string   `json:"path"`
	TypeName   string   `json:"type_name"`
	Purpose    string   `json:"purpose"`
	Platform   string   `json:"platform,omitempty"`
	Components string   `json:"components"`
	DataAccess string   `json:"data_access"`
	DependsOn  []string `json:"depends_on"`
}

// UnmarshalJSON accepts small planner-output variations (e.g. components as []string)
// and normalizes them into the canonical FilePlan schema used by the pipeline.
func (f *FilePlan) UnmarshalJSON(data []byte) error {
	type rawFilePlan struct {
		Path            string          `json:"path"`
		TypeName        string          `json:"type_name"`
		TypeNameCamel   string          `json:"typeName"`
		TypeAlias       string          `json:"type"`
		Purpose         string          `json:"purpose"`
		Platform        string          `json:"platform"`
		Components      json.RawMessage `json:"components"`
		DataAccess      string          `json:"data_access"`
		DataAccessCamel string          `json:"dataAccess"`
		DependsOn       json.RawMessage `json:"depends_on"`
		DependsOnCamel  json.RawMessage `json:"dependsOn"`
	}

	var raw rawFilePlan
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}

	f.Path = raw.Path
	f.TypeName = firstNonEmpty(raw.TypeName, raw.TypeNameCamel, raw.TypeAlias)
	f.Purpose = raw.Purpose
	f.Platform = raw.Platform
	f.DataAccess = firstNonEmpty(raw.DataAccess, raw.DataAccessCamel)

	components, err := decodeStringOrStringArray(raw.Components)
	if err != nil {
		return fmt.Errorf("invalid files.components: %w", err)
	}
	f.Components = components

	depRaw := raw.DependsOn
	if len(depRaw) == 0 {
		depRaw = raw.DependsOnCamel
	}
	dependsOn, err := decodeStringSliceOrString(depRaw)
	if err != nil {
		return fmt.Errorf("invalid files.depends_on: %w", err)
	}
	f.DependsOn = dependsOn

	return nil
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}

func decodeStringOrStringArray(raw json.RawMessage) (string, error) {
	if len(raw) == 0 || string(raw) == "null" {
		return "", nil
	}
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		return s, nil
	}
	var list []string
	if err := json.Unmarshal(raw, &list); err == nil {
		return strings.Join(list, "; "), nil
	}
	var anyList []any
	if err := json.Unmarshal(raw, &anyList); err == nil {
		parts := make([]string, 0, len(anyList))
		for _, v := range anyList {
			parts = append(parts, fmt.Sprint(v))
		}
		return strings.Join(parts, "; "), nil
	}
	return "", fmt.Errorf("expected string or array of strings")
}

func decodeStringSliceOrString(raw json.RawMessage) ([]string, error) {
	if len(raw) == 0 || string(raw) == "null" {
		return nil, nil
	}
	var list []string
	if err := json.Unmarshal(raw, &list); err == nil {
		return list, nil
	}
	var one string
	if err := json.Unmarshal(raw, &one); err == nil {
		one = strings.TrimSpace(one)
		if one == "" {
			return nil, nil
		}
		return []string{one}, nil
	}
	return nil, fmt.Errorf("expected []string or string")
}

// PlannedFileStatus tracks validation state for a planned file.
type PlannedFileStatus struct {
	PlannedPath  string
	ResolvedPath string
	ExpectedType string
	Exists       bool
	Valid        bool
	Reason       string
}

// FileCompletionReport summarizes completion coverage for planned files.
type FileCompletionReport struct {
	TotalPlanned int
	ValidCount   int
	Missing      []PlannedFileStatus
	Invalid      []PlannedFileStatus
	Complete     bool
}

// ModelPlan describes a data model in the build plan.
type ModelPlan struct {
	Name       string         `json:"name"`
	Storage    string         `json:"storage"`
	Properties []PropertyPlan `json:"properties"`
}

// UnmarshalJSON handles both object form {"name":"Foo",...} and bare string "Foo".
func (m *ModelPlan) UnmarshalJSON(data []byte) error {
	// Try bare string first
	var s string
	if err := json.Unmarshal(data, &s); err == nil {
		m.Name = s
		return nil
	}
	// Fall back to struct
	type alias ModelPlan
	var a alias
	if err := json.Unmarshal(data, &a); err != nil {
		return err
	}
	*m = ModelPlan(a)
	return nil
}

// PropertyPlan describes a single property on a model.
type PropertyPlan struct {
	Name         string `json:"name"`
	Type         string `json:"type"`
	DefaultValue string `json:"default_value"`
}

// Permission describes a required iOS permission.
type Permission struct {
	Key         string `json:"key"`
	Description string `json:"description"`
	Framework   string `json:"framework"`
}
