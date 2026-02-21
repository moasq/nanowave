package orchestration

// BuildResult is the final output of a successful Build pipeline.
type BuildResult struct {
	AppName          string
	ProjectDir       string
	BundleID         string
	Features         []string
	FileCount        int
	PlannedFiles     int
	CompletedFiles   int
	CompletionPasses int
	SessionID        string
	TotalCostUSD     float64
	InputTokens      int
	OutputTokens     int
	CacheRead        int
	CacheCreated     int
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
	Design        DesignSystem    `json:"design"`
	Files         []FilePlan      `json:"files"`
	Models        []ModelPlan     `json:"models"`
	Permissions   []Permission    `json:"permissions"`
	Extensions    []ExtensionPlan `json:"extensions"`
	Localizations []string        `json:"localizations"`
	RuleKeys      []string        `json:"rule_keys"`
	BuildOrder    []string        `json:"build_order"`
}

// ExtensionPlan describes a secondary Xcode target (widget, live activity, etc.)
type ExtensionPlan struct {
	Kind         string            `json:"kind"`
	Name         string            `json:"name"`
	Purpose      string            `json:"purpose"`
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
	Components string   `json:"components"`
	DataAccess string   `json:"data_access"`
	DependsOn  []string `json:"depends_on"`
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
