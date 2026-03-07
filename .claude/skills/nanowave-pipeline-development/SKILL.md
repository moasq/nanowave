---
name: nanowave-pipeline-development
description: "Use when modifying the build pipeline, adding new phases, changing prompt composition, or working with the Claude client integration."
---

# Pipeline Development Guide

## 6-Phase Pipeline

```
User Prompt
    ↓
1. Intent Router  → IntentDecision (advisory hints: operation, platform, device_family)
    ↓
2. Analyzer       → AnalysisResult (app_name, description, features, core_flow)
    ↓
3. Planner        → PlannerResult (design, files, models, permissions, extensions, rule_keys, build_order)
    ↓
4. Builder        → Generates code (streaming, up to 6 completion passes)
    ↓
5. Fixer          → Fixes build errors (iterative)
    ↓
6. Finalize       → Git init + commit
```

## Type Contracts

All phase outputs are parsed via `parseClaudeJSON[T]()` in `helpers.go`:

```go
// Generic parser: extracts JSON from markdown fences, unmarshals to T
func parseClaudeJSON[T any](result string, label string) (*T, error)

// JSON extraction: handles ```json fences and thinking text
func extractJSON(s string) string
```

### Phase-Specific Parsers

- `parseIntentDecision(result)` → validates operation, platform_hint, device_family_hint
- `parseAnalysis(result)` → validates app_name is non-empty
- `parsePlan(result)` → validates files non-empty, platform, extensions, rule_keys

## Prompt Composition

Located in `phase_prompts.go`:

```go
// Analyzer/Planner: full SystemPrompt (standalone phases)
composeAnalyzerSystemPrompt(intent) → base + constraints + phase skill + intent hints
composePlannerSystemPrompt(intent)  → base + constraints + phase skill + intent hints

// Builder/Fixer/Completion: AppendSystemPrompt (runs in workspace with CLAUDE.md)
composeCoderAppendPrompt(phaseSkillName) → coder base + shared constraints + phase skill
```

Key helper:
```go
func appendPromptSection(b *strings.Builder, title, content string)
```

### Why AppendSystemPrompt for Build Phase

The build phase runs inside the generated project workspace which has its own CLAUDE.md with design tokens, architecture rules, and memory modules. Using `AppendSystemPrompt` adds pipeline instructions without overriding the workspace CLAUDE.md.

## Claude Client Integration

```go
type GenerateOpts struct {
    SystemPrompt       string   // Full system prompt (analyzer, planner)
    AppendSystemPrompt string   // Appended to workspace CLAUDE.md (builder, fixer)
    MaxTurns           int
    Model              string
    WorkDir            string
    AllowedTools       []string // e.g., agenticTools
    SessionID          string   // For multi-pass session continuity
}

// Streaming generation with progress callback
client.GenerateStreaming(ctx, prompt, opts, progressCallback)
```

## Build Phase Details

In `pipeline.go`:
- Up to `maxBuildCompletionPasses` (6) completion passes
- Deterministic file completion gate via `FileCompletionReport`
- Plateau detection: if no new files complete across passes, terminate early
- `retryPhase[T]()` handles transient API failures with exponential backoff

In `build_prompts.go`:
- `buildPrompts()` — constructs append prompt + user message with plan context
- `completionPrompts()` — targeted prompts for unresolved files only
- Multi-platform vs single-platform variants for build commands

## Adding a New Phase

1. Create skill directory: `internal/orchestration/skills/phases/{phase-name}/`
2. Add `SKILL.md` with Anthropic frontmatter + phase instructions
3. Add `references/` for workflow, output-format, common-mistakes, examples
4. Create compose function in `phase_prompts.go`:
   ```go
   func composeNewPhasePrompt() (string, error) {
       phaseSkill, err := loadPhaseSkillContent("phase-name")
       // ...
   }
   ```
5. Wire into `pipeline.go` — add method to Pipeline struct
6. Define output type in `types.go` if needed
7. Add parser in `helpers.go` using `parseClaudeJSON[T]()`
8. Run `make skills-source-validate` and `make test`

## Available Tools for Build Phase

Defined as `agenticTools` in `pipeline.go`:
- Write, Edit, Read, Bash, Glob, Grep — file operations
- WebFetch, WebSearch — research (rate-limited)
- Apple Docs MCP — framework documentation
- XcodeGen MCP — project configuration
