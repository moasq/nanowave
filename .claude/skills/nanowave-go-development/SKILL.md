---
name: nanowave-go-development
description: "Use when writing, reviewing, or modifying Go code in the nanowave CLI. Covers Go 1.26 conventions, testing patterns, cobra commands, and the string matching policy."
---

# Go Development Conventions

## Go Version & Module

- Go 1.26, module `github.com/moasq/nanowave`
- Use `go vet ./...` before committing
- Use `go test ./... -v` to run all tests

## Code Style

- Error wrapping: always use `fmt.Errorf("context: %w", err)` for wrapped errors
- Prefer early returns over deep nesting
- Use `strings.Builder` for string concatenation in loops
- No global mutable state — pass dependencies explicitly

## Testing Patterns

- Table-driven tests with `t.Run()` subtests:
```go
tests := []struct {
    name string
    input string
    want  string
}{
    {"basic", "hello", "Hello"},
    {"empty", "", ""},
}
for _, tt := range tests {
    t.Run(tt.name, func(t *testing.T) {
        got := myFunc(tt.input)
        if got != tt.want {
            t.Errorf("myFunc(%q) = %q, want %q", tt.input, got, tt.want)
        }
    })
}
```
- Use `t.TempDir()` for filesystem tests — auto-cleaned after test
- Use `t.Helper()` in test helper functions
- Test file naming: `foo_test.go` alongside `foo.go`

## String Matching Policy

### ALLOWED — finite known sets

```go
// Platform constants (3 values, defined in platform_features.go)
switch platform {
case PlatformIOS:
    // ...
case PlatformWatchOS:
    // ...
case PlatformTvOS:
    // ...
}

// Map lookup on finite set of rule keys
if _, unsupported := watchOSUnsupportedRuleKeys[key]; unsupported {
    warnings = append(warnings, key)
}

// Known file extension
if strings.HasSuffix(name, ".swift") { ... }

// Finite operation values from our own schema
switch decision.Operation {
case "build", "edit", "fix":
    // valid
default:
    decision.Operation = "build"
}
```

### BANNED — open-ended/unbounded input

```go
// NEVER: regex on user prompt (unbounded input)
if regexp.MustCompile("(?i)weather|forecast").MatchString(userPrompt) { ... }

// NEVER: Contains on user description (unbounded)
if strings.Contains(featureDescription, "camera") { ... }

// NEVER: regex-based feature detection on AI output
matched := regexp.MustCompile("(?i)chart|graph|plot").MatchString(analysisText)
```

**Rule**: If the value set is defined in our constants/maps, string matching is OK. If the input is user-provided or AI-generated with unlimited possible values, use typed contracts and `parseClaudeJSON[T]()`.

## Adding New Commands

1. Create `internal/commands/newcmd.go`
2. Define cobra command with `Use`, `Short`, `Long`, `RunE`
3. Register in `internal/commands/root.go` via `rootCmd.AddCommand()`
4. Follow existing patterns in `run.go`, `fix.go`, `info.go`

## Adding New Rule Keys

1. Add the rule key to appropriate maps in `platform_features.go`
2. Create skill file: `internal/orchestration/skills/features/{key}/SKILL.md`
3. Add references in `references/` subdirectory if needed
4. Loading is automatic via `loadRuleContent()` in `setup.go`
5. Run `make skills-source-validate` to verify compliance

## Adding New Platform Support

1. Add constant in `platform_features.go` (e.g., `PlatformVisionOS = "visionos"`)
2. Add unsupported rule key map, conditional rule map, unsupported extension set
3. Implement `Validate*`, `Platform*` helper functions for the new platform
4. Add `always-{platform}/` skill directory with platform-specific skills
5. Update `setup.go` for build commands and memory file generation
6. Add test cases in `platform_features_test.go`
