package orchestration

import "testing"

func TestGetDeviceFamilyMacOSReturnsEmpty(t *testing.T) {
	p := &PlannerResult{Platform: PlatformMacOS}
	if got := p.GetDeviceFamily(); got != "" {
		t.Errorf("macOS GetDeviceFamily() = %q, want empty", got)
	}
}

func TestGetDeviceFamilyTvOSReturnsEmpty(t *testing.T) {
	p := &PlannerResult{Platform: PlatformTvOS}
	if got := p.GetDeviceFamily(); got != "" {
		t.Errorf("tvOS GetDeviceFamily() = %q, want empty", got)
	}
}

func TestGetDeviceFamilyVisionOSReturnsEmpty(t *testing.T) {
	p := &PlannerResult{Platform: PlatformVisionOS}
	if got := p.GetDeviceFamily(); got != "" {
		t.Errorf("visionOS GetDeviceFamily() = %q, want empty", got)
	}
}

func TestGetDeviceFamilyWatchOSReturnsEmpty(t *testing.T) {
	p := &PlannerResult{Platform: PlatformWatchOS}
	if got := p.GetDeviceFamily(); got != "" {
		t.Errorf("watchOS GetDeviceFamily() = %q, want empty", got)
	}
}

func TestGetDeviceFamilyIOSDefaultsToIphone(t *testing.T) {
	p := &PlannerResult{Platform: PlatformIOS}
	if got := p.GetDeviceFamily(); got != "iphone" {
		t.Errorf("iOS GetDeviceFamily() = %q, want \"iphone\"", got)
	}
}

func TestGetDeviceFamilyNilDefaultsToIphone(t *testing.T) {
	var p *PlannerResult
	if got := p.GetDeviceFamily(); got != "iphone" {
		t.Errorf("nil GetDeviceFamily() = %q, want \"iphone\"", got)
	}
}

func TestGetDeviceFamilyExplicitValue(t *testing.T) {
	p := &PlannerResult{Platform: PlatformIOS, DeviceFamily: "universal"}
	if got := p.GetDeviceFamily(); got != "universal" {
		t.Errorf("explicit GetDeviceFamily() = %q, want \"universal\"", got)
	}
}

func TestHasRuleKey(t *testing.T) {
	p := &PlannerResult{RuleKeys: []string{"liquid-glass", "dark-mode", "storage"}}
	if !p.HasRuleKey("dark-mode") {
		t.Error("expected HasRuleKey(\"dark-mode\") = true")
	}
	if p.HasRuleKey("animations") {
		t.Error("expected HasRuleKey(\"animations\") = false")
	}
}

func TestHasRuleKeyNilPlan(t *testing.T) {
	var p *PlannerResult
	if p.HasRuleKey("dark-mode") {
		t.Error("expected nil plan HasRuleKey = false")
	}
}
