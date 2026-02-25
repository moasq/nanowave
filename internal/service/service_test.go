package service

import "testing"

func TestPlatformBundleIDSuffix(t *testing.T) {
	tests := []struct {
		platform string
		want     string
	}{
		{"ios", ""},
		{"watchos", ""},
		{"tvos", ".tv"},
		{"visionos", ".vision"},
		{"macos", ".mac"},
	}
	for _, tt := range tests {
		t.Run(tt.platform, func(t *testing.T) {
			got := platformBundleIDSuffix(tt.platform)
			if got != tt.want {
				t.Errorf("platformBundleIDSuffix(%q) = %q, want %q", tt.platform, got, tt.want)
			}
		})
	}
}
