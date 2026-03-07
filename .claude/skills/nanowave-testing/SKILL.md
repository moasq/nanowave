---
name: nanowave-testing
description: "Use when writing, running, or debugging tests for the nanowave CLI. Covers test framework, coverage map, patterns, and commands."
---

# Testing Guide

## Test Framework

- Standard `testing` package — no external test frameworks
- Table-driven tests with `t.Run()` subtests
- `t.TempDir()` for filesystem tests (auto-cleanup)
- `t.Helper()` in test helper functions

## Running Tests

```bash
# All tests
go test ./... -v

# Specific package
go test ./internal/orchestration/ -v

# Specific test
go test ./internal/orchestration/ -v -run TestExtractJSON

# Skill compliance only
make skills-source-validate

# Full validation
make build && make test && make skills-source-validate
```

## Coverage Map

### Covered (has tests)

| Area | Test File | Key Tests |
|------|-----------|-----------|
| Platform features | `platform_features_test.go` | ValidatePlatform, FilterRuleKeysForPlatform, extension validation |
| Skill compliance | `skill_compliance_test.go` | TestSourceSkillsAnthropicComplianceStrict (used by `make skills-source-validate`) |
| Intent router | `intent_router_test.go` | Intent decision parsing, operation validation |
| Setup/workspace | `setup_test.go` | Workspace creation, CLAUDE.md generation |
| XcodeGen | `xcodegen_test.go` | Config generation, platform settings |
| File completion | `completion_test.go` | PlannedFileStatus resolution, FileCompletionReport |
| Parser tolerance | `parser_tolerance_test.go` | Malformed JSON handling, fence extraction |
| Watch shape | `watch_shape_intent_test.go` | Watch project shape detection |
| Phase prompts | `phase_prompts_test.go` | Prompt composition, section assembly |

### Coverage Gaps

| Area | File | What Needs Tests |
|------|------|-----------------|
| JSON helpers | `helpers.go` | `extractJSON` (plain, fence, thinking), `bundleIDPrefix`, `sanitizeToPascalCase`, `uniqueProjectDir` |
| Build prompts | `build_prompts.go` | `buildPrompts()` sections present, `completionPrompts()` lists unresolved files |
| Pipeline integration | `pipeline.go` | End-to-end pipeline flow (requires mock Claude client) |

## Test Patterns

### JSON Parsing Tests
```go
func TestExtractJSON(t *testing.T) {
    tests := []struct {
        name  string
        input string
        want  string
    }{
        {"plain JSON", `{"key":"value"}`, `{"key":"value"}`},
        {"markdown fence", "```json\n{\"key\":\"value\"}\n```", `{"key":"value"}`},
        {"thinking + fence", "Let me think...\n```json\n{\"key\":\"value\"}\n```\nDone.", `{"key":"value"}`},
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
```

### Filesystem Tests
```go
func TestUniqueProjectDir(t *testing.T) {
    dir := t.TempDir()
    // First call returns base path
    got := uniqueProjectDir(dir, "MyApp")
    // Create it to force collision
    os.MkdirAll(got, 0o755)
    // Second call appends counter
    got2 := uniqueProjectDir(dir, "MyApp")
    if got == got2 {
        t.Error("expected different paths for collision")
    }
}
```

### Prompt Tests
```go
func TestBuildPromptsContainsSections(t *testing.T) {
    // Create minimal Pipeline + plan
    // Call buildPrompts()
    // Check output contains "## Build Plan", "### Design", "### Models", "### Files"
}
```
