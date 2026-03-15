# Nanowave CLI — Development Guide

Nanowave is an AI-powered Apple app generator CLI written in Go 1.26.
Module: `github.com/moasq/nanowave`

## Architecture

```
cmd/nanowave/          → CLI entry point (cobra)
internal/
  commands/            → Cobra command definitions (root, setup, interactive, run, fix, info, mcp)
  orchestration/       → Build pipeline (intent → analyze → plan → build → finalize)
    pipeline.go        → Primary orchestrator (Pipeline struct, Build/Edit methods)
    setup.go           → Workspace setup, CLAUDE.md, build commands
    setup_skills.go    → Core rules writing + rule content loading
    setup_claudemd.go  → Generated Makefile
    build_prompts.go   → Build-phase prompt construction
    phase_prompts.go   → Prompt composition (composeAnalyzerSystemPrompt, etc.)
    helpers.go         → JSON parsing (parseClaudeJSON[T], extractJSON), utilities
    types.go           → Type contracts (IntentDecision, AnalysisResult, PlannerResult, BuildResult)
    platform_features.go → Platform constants + validation (iOS, watchOS, tvOS)
    completion.go      → File completion gate (PlannedFileStatus, FileCompletionReport)
    intent_router.go   → Pre-analysis intent detection
    exports.go         → External API for nwtool/service packages
  skills/              → Embedded skill files (//go:embed data)
    data/core/         → Core rules (always copied to .claude/rules/)
    data/always/       → Always-on skills (feature content)
    data/features/     → Feature-specific content
    data/ui/           → UI-specific content
    data/extensions/   → Extension-specific content
  nwtool/              → Agent tool registry (nw_setup_workspace, etc.)
  asc/                 → App Store Connect types, credentials, agreements, bundle ID, iris API
  appleauth/           → Apple ID SRP authentication, 2FA, onboarding, session cookies
  icons/               → App icon discovery, resizing, Contents.json generation, upload server
  ascserver/           → ASC MCP server
  mcpregistry/         → Internal MCP server registry (apple-docs, xcodegen, asc)
  claude/              → Claude API client (GenerateStreaming, StreamEvent)
  config/              → CLI configuration management
  terminal/            → Terminal UI (ProgressDisplay, spinner, colors)
  storage/             → Data persistence
  service/             → Service utilities
  xcodegenserver/      → XcodeGen MCP server
```

## Critical Rules

### String Matching Policy

**ALLOWED** — known finite sets:
- `switch platform { case PlatformIOS, PlatformWatchOS, PlatformTvOS: }` (3 platform constants)
- `watchOSUnsupportedRuleKeys[key]` (map lookup on finite set)
- `strings.HasSuffix(name, ".swift")` (known file extension check)
- `switch decision.Operation { case "build", "edit", "fix": }` (3 operation constants)

**BANNED** — open-ended/unbounded input:
- `strings.Contains(userPrompt, "watch")` — never parse user prompts with string matching
- `regexp.MustCompile("(?i)chart|graph|plot").MatchString(desc)` — never detect features via regex on descriptions
- `if strings.Contains(featureDescription, "camera")` — unbounded feature descriptions are not finite sets

**Rule of thumb**: If the set of possible values is defined in our code (constants, map keys), string matching is fine. If the input comes from users or AI output with unlimited possible values, use typed contracts and structured parsing instead.

### Type-Safe Detection

- Use typed constants + switch/map for all detection logic
- Use `parseClaudeJSON[T]()` for all structured Claude output — never raw string manipulation
- Phase contracts: `IntentDecision`, `AnalysisResult`, `PlannerResult`, `BuildResult` define exact JSON shapes

### AppTheme Enforcement

- Generated apps must use `AppTheme` tokens for all colors, fonts, spacing
- Never hardcode `Color(...)`, `.font(.system(...))`, or magic padding numbers
- Reference `skills/data/core/forbidden-patterns.md` for full forbidden pattern list

### Prompt Composition

- Build phase uses `AppendSystemPrompt` (not `SystemPrompt`) — it runs in workspace with CLAUDE.md
- All structured output parsed via `parseClaudeJSON[T]()` with `extractJSON()` fence handling
- `composeAnalyzerSystemPrompt()`, `composePlannerSystemPrompt()`, `composeCoderAppendPrompt()` compose prompts using `appendPromptSection()`
- No phase skill loading — prompts use inline base constants + constraints

## Development Workflow

```bash
# Build the CLI
make build

# Run all tests
make test

# Lint
go vet ./...

# Run specific test
go test ./internal/orchestration/ -v -run TestName

# Build + test (full check)
make build && make test
```

## Key Files Reference

| Area | Files |
|------|-------|
| Pipeline entry | `pipeline.go` — `Build()`, `Edit()` methods |
| JSON parsing | `helpers.go` — `parseClaudeJSON[T]()`, `extractJSON()`, `sanitizeToPascalCase()` |
| Type contracts | `types.go` — all phase input/output structs |
| Platform logic | `platform_features.go` — `ValidatePlatform()`, `FilterRuleKeysForPlatform()` |
| Core rules | `setup_skills.go` — `writeCoreRules()`, `loadRuleContent()` |
| Prompt composition | `phase_prompts.go` — `appendPromptSection()`, prompt composers |
| Build prompts | `build_prompts.go` — `buildPrompts()`, `completionPrompts()` |
| Intent routing | `intent_router.go` — `composeIntentRouterSystemPrompt()` |
| Completion gate | `completion.go` — `PlannedFileStatus`, `FileCompletionReport` |
| Workspace setup | `setup.go` — `setupWorkspace()`, `writeInitialCLAUDEMD()` |
| External API | `exports.go` — exported wrappers for nwtool/service |
