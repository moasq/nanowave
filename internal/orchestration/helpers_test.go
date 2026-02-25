package orchestration

import (
	"os"
	"path/filepath"
	"testing"
)

func TestExtractJSON(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "plain JSON object",
			input: `{"key":"value"}`,
			want:  `{"key":"value"}`,
		},
		{
			name:  "markdown json fence",
			input: "```json\n{\"key\":\"value\"}\n```",
			want:  `{"key":"value"}`,
		},
		{
			name:  "plain fence without language",
			input: "```\n{\"key\":\"value\"}\n```",
			want:  `{"key":"value"}`,
		},
		{
			name:  "thinking text before fence",
			input: "Let me think about this...\n\n```json\n{\"app_name\":\"Skies\"}\n```\n\nDone thinking.",
			want:  `{"app_name":"Skies"}`,
		},
		{
			name:  "JSON with surrounding whitespace",
			input: "  \n  {\"a\":1}  \n  ",
			want:  `{"a":1}`,
		},
		{
			name:  "nested JSON objects",
			input: `{"outer":{"inner":"value"}}`,
			want:  `{"outer":{"inner":"value"}}`,
		},
		{
			name:  "no JSON at all",
			input: "just some text",
			want:  "just some text",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractJSON(tt.input)
			if got != tt.want {
				t.Errorf("extractJSON() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestParseClaudeJSON(t *testing.T) {
	type simple struct {
		Name  string `json:"name"`
		Value int    `json:"value"`
	}

	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{
			name:    "valid plain JSON",
			input:   `{"name":"test","value":42}`,
			wantErr: false,
		},
		{
			name:    "valid fenced JSON",
			input:   "```json\n{\"name\":\"test\",\"value\":42}\n```",
			wantErr: false,
		},
		{
			name:    "invalid JSON",
			input:   `{not json}`,
			wantErr: true,
		},
		{
			name:    "empty string",
			input:   "",
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := parseClaudeJSON[simple](tt.input, "test")
			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if result.Name != "test" || result.Value != 42 {
				t.Errorf("unexpected result: %+v", result)
			}
		})
	}
}

func TestSanitizeBundleID(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"simple", "janedoe", "janedoe"},
		{"with hyphens", "jane-doe", "janedoe"},
		{"with dots", "jane.doe", "janedoe"},
		{"uppercase", "JaneDoe", "janedoe"},
		{"with spaces", "jane doe", "janedoe"},
		{"with numbers", "jane123", "jane123"},
		{"special chars", "j@ne!d#e", "jnede"},
		{"empty", "", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := sanitizeBundleID(tt.input)
			if got != tt.want {
				t.Errorf("sanitizeBundleID(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestSanitizeToPascalCase(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"simple lowercase", "hello", "Hello"},
		{"already pascal", "Hello", "Hello"},
		{"with spaces", "hello world", "HelloWorld"},
		{"with hyphens", "my-app", "MyApp"},
		{"with underscores", "my_app", "MyApp"},
		{"mixed case spaces", "my Cool App", "MyCoolApp"},
		{"numbers", "app2go", "App2go"},
		{"leading number", "2fast", "2fast"},
		{"empty", "", ""},
		{"special chars only", "---", ""},
		{"single word", "skies", "Skies"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := sanitizeToPascalCase(tt.input)
			if got != tt.want {
				t.Errorf("sanitizeToPascalCase(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestUniqueProjectDir(t *testing.T) {
	dir := t.TempDir()

	// First call returns base path (no collision)
	got1 := uniqueProjectDir(dir, "MyApp")
	wantBase := filepath.Join(dir, "MyApp")
	if got1 != wantBase {
		t.Errorf("first call = %q, want %q", got1, wantBase)
	}

	// Create the directory to force collision
	if err := os.MkdirAll(got1, 0o755); err != nil {
		t.Fatal(err)
	}

	// Second call should return MyApp2
	got2 := uniqueProjectDir(dir, "MyApp")
	want2 := filepath.Join(dir, "MyApp2")
	if got2 != want2 {
		t.Errorf("second call = %q, want %q", got2, want2)
	}

	// Create MyApp2 too
	if err := os.MkdirAll(got2, 0o755); err != nil {
		t.Fatal(err)
	}

	// Third call should return MyApp3
	got3 := uniqueProjectDir(dir, "MyApp")
	want3 := filepath.Join(dir, "MyApp3")
	if got3 != want3 {
		t.Errorf("third call = %q, want %q", got3, want3)
	}
}

func TestTruncateStr(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		maxLen int
		want   string
	}{
		{"short", "hello", 10, "hello"},
		{"exact", "hello", 5, "hello"},
		{"over", "hello world", 5, "hello..."},
		{"empty", "", 5, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := truncateStr(tt.input, tt.maxLen)
			if got != tt.want {
				t.Errorf("truncateStr(%q, %d) = %q, want %q", tt.input, tt.maxLen, got, tt.want)
			}
		})
	}
}

func TestExtractSpinnerStatus(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"empty", "", ""},
		{"whitespace", "   ", ""},
		{"single sentence", "Creating the model file", "Creating the model file"},
		{"sentence with period", "Creating the model file. Then the view.", "Creating the model file"},
		{"with newline", "First line\nSecond line", "First line"},
		{"long text truncated", "This is a very long status message that exceeds the maximum width allowed for spinner display and should be truncated", "This is a very long status message that exceeds the maximum ..."},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractSpinnerStatus(tt.input)
			if got != tt.want {
				t.Errorf("extractSpinnerStatus(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestParsePlan_WithPackages(t *testing.T) {
	plan := `{
		"design": {"navigation":"tabs","palette":{"primary":"#FF0000","secondary":"#00FF00","accent":"#0000FF","background":"#FFFFFF","surface":"#F0F0F0"},"font_design":"default","corner_radius":12,"density":"standard","surfaces":"solid","app_mood":"calm"},
		"platform": "ios",
		"device_family": "iphone",
		"files": [{"path":"App/MyApp.swift","type_name":"MyApp","purpose":"entry","components":"@main App","data_access":"none","depends_on":[]}],
		"models": [],
		"permissions": [],
		"extensions": [],
		"localizations": [],
		"rule_keys": [],
		"packages": [
			{"name": "Lottie", "reason": "Complex vector animations not possible with native SwiftUI"},
			{"name": "SDWebImageSwiftUI", "reason": "Efficient async image caching beyond AsyncImage"}
		],
		"build_order": ["App/MyApp.swift"]
	}`

	result, err := parsePlan(plan)
	if err != nil {
		t.Fatalf("parsePlan() error: %v", err)
	}
	if len(result.Packages) != 2 {
		t.Fatalf("expected 2 packages, got %d", len(result.Packages))
	}
	if result.Packages[0].Name != "Lottie" {
		t.Errorf("expected first package name Lottie, got %q", result.Packages[0].Name)
	}
	if result.Packages[1].Reason != "Efficient async image caching beyond AsyncImage" {
		t.Errorf("unexpected second package reason: %q", result.Packages[1].Reason)
	}
}

func TestParsePlan_EmptyNamePackageDropped(t *testing.T) {
	plan := `{
		"design": {"navigation":"tabs","palette":{"primary":"#FF0000","secondary":"#00FF00","accent":"#0000FF","background":"#FFFFFF","surface":"#F0F0F0"},"font_design":"default","corner_radius":12,"density":"standard","surfaces":"solid","app_mood":"calm"},
		"platform": "ios",
		"device_family": "iphone",
		"files": [{"path":"App/MyApp.swift","type_name":"MyApp","purpose":"entry","components":"@main App","data_access":"none","depends_on":[]}],
		"models": [],
		"permissions": [],
		"extensions": [],
		"localizations": [],
		"rule_keys": [],
		"packages": [
			{"name": "Lottie", "reason": "animations"},
			{"name": "", "reason": "no name"},
			{"name": "  ", "reason": "whitespace only"}
		],
		"build_order": ["App/MyApp.swift"]
	}`

	result, err := parsePlan(plan)
	if err != nil {
		t.Fatalf("parsePlan() error: %v", err)
	}
	if len(result.Packages) != 1 {
		t.Fatalf("expected 1 package after dropping empty names, got %d", len(result.Packages))
	}
	if result.Packages[0].Name != "Lottie" {
		t.Errorf("expected remaining package to be Lottie, got %q", result.Packages[0].Name)
	}
}

func TestParsePlan_DuplicatePackagesDeduped(t *testing.T) {
	plan := `{
		"design": {"navigation":"tabs","palette":{"primary":"#FF0000","secondary":"#00FF00","accent":"#0000FF","background":"#FFFFFF","surface":"#F0F0F0"},"font_design":"default","corner_radius":12,"density":"standard","surfaces":"solid","app_mood":"calm"},
		"platform": "ios",
		"device_family": "iphone",
		"files": [{"path":"App/MyApp.swift","type_name":"MyApp","purpose":"entry","components":"@main App","data_access":"none","depends_on":[]}],
		"models": [],
		"permissions": [],
		"extensions": [],
		"localizations": [],
		"rule_keys": [],
		"packages": [
			{"name": "Lottie", "reason": "first"},
			{"name": "Lottie", "reason": "duplicate"}
		],
		"build_order": ["App/MyApp.swift"]
	}`

	result, err := parsePlan(plan)
	if err != nil {
		t.Fatalf("parsePlan() error: %v", err)
	}
	if len(result.Packages) != 1 {
		t.Fatalf("expected 1 package after dedup, got %d", len(result.Packages))
	}
}

func TestParsePlan_NilPackagesInitialized(t *testing.T) {
	plan := `{
		"design": {"navigation":"tabs","palette":{"primary":"#FF0000","secondary":"#00FF00","accent":"#0000FF","background":"#FFFFFF","surface":"#F0F0F0"},"font_design":"default","corner_radius":12,"density":"standard","surfaces":"solid","app_mood":"calm"},
		"platform": "ios",
		"device_family": "iphone",
		"files": [{"path":"App/MyApp.swift","type_name":"MyApp","purpose":"entry","components":"@main App","data_access":"none","depends_on":[]}],
		"models": [],
		"permissions": [],
		"extensions": [],
		"localizations": [],
		"rule_keys": [],
		"build_order": ["App/MyApp.swift"]
	}`

	result, err := parsePlan(plan)
	if err != nil {
		t.Fatalf("parsePlan() error: %v", err)
	}
	if result.Packages == nil {
		t.Fatal("expected Packages to be initialized to empty slice, got nil")
	}
	if len(result.Packages) != 0 {
		t.Fatalf("expected 0 packages, got %d", len(result.Packages))
	}
}

func TestExtractToolInputString(t *testing.T) {
	tests := []struct {
		name  string
		input string
		key   string
		want  string
	}{
		{"valid key", `{"file_path":"/tmp/test.swift","content":"hello"}`, "file_path", "/tmp/test.swift"},
		{"missing key", `{"file_path":"/tmp/test.swift"}`, "other", ""},
		{"non-string value", `{"count":42}`, "count", ""},
		{"empty input", ``, "key", ""},
		{"invalid JSON", `{invalid}`, "key", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractToolInputString([]byte(tt.input), tt.key)
			if got != tt.want {
				t.Errorf("extractToolInputString(%q, %q) = %q, want %q", tt.input, tt.key, got, tt.want)
			}
		})
	}
}
