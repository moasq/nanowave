# Vibeship CLI: Open-Source iOS App Builder Powered by Claude Code

## The Pitch

> Rork charges $200+. Vibeship is free and open source. You just need a Claude Code subscription (which you're already paying for). One command: `vibeship build "habit tracker app"` — working Xcode project on your Mac.

## Architecture Overview

The current backend is a SaaS server: PostgreSQL, Temporal, Redis, WebSocket, auth, billing. The CLI strips all of that. The orchestration engine (analyzer → planner → builder → editor → fixer → deployer) is the same — it just runs locally on the user's Mac, uses Claude Code for LLM calls instead of a raw Anthropic API key, and writes files to disk instead of sending them over WebSocket.

```
BEFORE (SaaS):
  Client (macOS app)
    ↕ WebSocket + ACTION_REQUEST/RESULT
  Server (Go API)
    → PostgreSQL (projects, messages, project_index, conversation_summary)
    → Temporal (async summary, repo map, procedures workflows)
    → LanceDB (semantic search embeddings)
    → Voyage AI (embedding generation)
    → Redis (caching)
    → Anthropic API (direct, your key)

AFTER (CLI):
  Terminal
    → vibeship CLI (Go binary, runs locally)
      → Claude Code SDK (user's subscription, subprocess)
      → Local JSON files (.vibeship/ directory)
      → xcodegen + xcodebuild (user's Mac)
      → Tree-sitter (repo map, runs locally)
      → No server, no database, no auth, no billing
```

---

## Component-by-Component Migration

### 1. PostgreSQL → Local JSON Files

**What PostgreSQL stores today:**

| Table | Purpose | CLI Replacement |
|---|---|---|
| `ios_agent.projects` | Project metadata (name, path, bundle_id, conversation_summary) | `.vibeship/project.json` |
| `ios_agent.project_index` | File descriptions, models, design, procedures, repo_map (JSONB) | `.vibeship/project_index.json` |
| `ios_agent.messages` | Conversation history (prompt + response + phases) | `.vibeship/history.json` |

**`project.json`** — replaces the `projects` table row:
```json
{
  "name": "HabitTracker",
  "bundle_id": "com.vibeship.HabitTracker",
  "workspace_status": "ready",
  "conversation_summary": "User built a habit tracker with streaks and notifications...",
  "created_at": "2025-06-15T10:30:00Z"
}
```

**`project_index.json`** — replaces the `project_index` table row:
```json
{
  "file_descriptions": [
    {"path": "ContentView.swift", "type_name": "ContentView", "purpose": "Main tab navigation", "components": "...", "depends_on": ["HabitListView"]},
    {"path": "Features/Habits/HabitListView.swift", "type_name": "HabitListView", "purpose": "Displays habits with streak indicators", "components": "..."}
  ],
  "models": [
    {"name": "Habit", "storage": "SwiftData", "properties": "name: String, streak: Int, lastCompleted: Date"}
  ],
  "design": {"color_scheme": "blue/orange", "style": "rounded cards"},
  "repo_map": "# Ranked file summary\n1. ContentView.swift [importance=0.95]\n  struct ContentView: View {...}",
  "procedures": [
    {"id": "add_language", "trigger": "User wants to add a new language", "steps": "1. Add to Localizations in project.json\n2. Create xx.lproj/Localizable.strings\n3. ..."}
  ]
}
```

**`history.json`** — replaces the `messages` table:
```json
{
  "messages": [
    {"role": "user", "content": "Build me a habit tracker with streaks", "timestamp": "..."},
    {"role": "assistant", "content": "I'll create a HabitTracker app with...", "timestamp": "..."}
  ]
}
```

**Implementation**: New package `internal/storage/local.go` implements the same domain interfaces (`ProjectRepository`, `ProjectIndexRepository`, `MessageRepository`) but reads/writes JSON files from `.vibeship/` directory. The orchestration layer doesn't know or care — it depends on interfaces, not PostgreSQL.

### 2. LanceDB + Voyage AI (Embeddings) → DROP

**What it does today**: Semantic vector search over file descriptions and symbols. The locator uses it to find relevant files when editing. Voyage AI generates 1536-dim embeddings, LanceDB stores and queries them.

**Why drop it for CLI**:
- Claude Code already has web search + file reading built in
- The repo map (tree-sitter) + `find_symbol` tool + `search_code` tool already give the locator/fixer good code navigation
- Embeddings require a Voyage API key ($) — contradicts the "free" pitch
- LanceDB adds a CGO dependency (Apache Arrow) that makes cross-compilation painful
- For a 15-25 file iOS app, keyword search is sufficient. Semantic search is overkill.

**What changes**:
- `domain.VectorStoreRepository` stays as interface but implementation is always `nil`
- `domain.EmbedderService` stays as interface but implementation is always `nil`
- `infra/lancedb/` package excluded from CLI build (build tag or just not imported)
- `infra/voyage/` package excluded from CLI build
- Orchestrator already handles `nil` gracefully: `if o.vectorStore != nil && o.embedder != nil { ... }` — no code change needed
- `SemanticSearchTool` on `RuntimeState` stays nil — locator/fixer already work without it

**Future option**: If someone wants embeddings, they can set `VOYAGE_API_KEY` and we could bring it back as an optional plugin. But for v1, zero dependencies.

### 3. Repo Map (Tree-Sitter) → KEEP AS-IS

**What it does**: After each build/edit, generates a ranked symbol index of all Swift files using tree-sitter AST parsing. The locator and fixer use `find_symbol` tool to navigate code.

**Why keep it**:
- Zero external dependencies (Go tree-sitter library, compiled in)
- Runs locally on disk files — no network, no database
- Critical for edit/fix flow quality — without it, the locator has to read every file

**What changes**:
- Instead of Temporal workflow → activity → store in PostgreSQL, it runs **synchronously** after deploy
- Output stored in `.vibeship/project_index.json` (the `repo_map` field)
- Same `infra/treesitter/` package, same `RepoMapBuilder` — just called directly instead of via Temporal

```go
// Before (async Temporal workflow):
n.triggerRepoMapGeneration(ctx, project.ID, appPath)

// After (synchronous, local):
repoMap, err := repoMapBuilder.Build(ctx, appSourceDir)
if err == nil {
    indexData.RepoMap = repoMap
    localStore.SaveProjectIndex(indexData)
}
```

### 4. Temporal Workflows → Synchronous Local Execution

**What Temporal does today**: Three async workflows fire after each build/edit:

| Workflow | Purpose | CLI Replacement |
|---|---|---|
| `SummarizeConversationWorkflow` | Haiku summarizes conversation history | Run inline after each command (or skip — Claude Code maintains its own context) |
| `BuildRepoMapWorkflow` | Tree-sitter generates symbol index + optional embeddings | Run synchronously after deploy |
| `GenerateProceduresWorkflow` | Haiku generates modification procedures from file descriptions | Run synchronously after deploy |

**Why not keep Temporal**: It's a distributed workflow engine. A CLI running on one machine doesn't need it. The workflows take 2-5 seconds each — running them synchronously is fine.

**What changes**:
- Delete all Temporal imports and workflow/activity registrations
- The activities themselves (`RepoMapActivity.GenerateRepoMap`, `ProceduresActivity.GenerateProcedures`, `SummaryActivity.GenerateSummary`) contain the actual logic. Extract the logic, call it directly.
- The `service.go` `triggerSummaryUpdate()` becomes `generateSummary()` — a direct function call

```go
// Before (Temporal):
s.temporalClient.StartWorkflow(ctx, opts, SummarizeConversationWorkflow, input)

// After (direct call):
summary, err := s.summaryGenerator.Generate(ctx, messages)
if err == nil {
    localStore.SaveConversationSummary(summary)
}
```

### 5. Anthropic API (Direct) → Claude Code SDK

**What happens today**: The `provider.go` creates Claude models via `claude.NewChatModel()` using `ANTHROPIC_API_KEY` env var. Each node (router, analyzer, planner, builder, etc.) gets its own model instance with specific `MaxTokens`.

**The pivot**: Instead of calling the Anthropic API directly, invoke Claude Code as a subprocess. The user's Claude Code subscription handles billing.

**Two integration approaches:**

#### Option A: Claude Code SDK (TypeScript subprocess) — RECOMMENDED
```go
// Go wraps the Claude Code SDK via subprocess
func (m *ClaudeCodeModel) Generate(ctx context.Context, messages []*schema.Message, opts ...model.Option) (*schema.Message, error) {
    // Serialize messages to JSON
    // Invoke: claude --print --output-format json --system-prompt <system> --message <user>
    // Parse JSON response
    // Return as schema.Message
}
```

**Pros**: Uses the user's Claude Code subscription directly. No API key needed. Supports all Claude Code features (web search, file access).

**Cons**: Subprocess overhead (~200ms per call). Need to handle streaming differently.

#### Option B: Raw API with User's Key
```go
// User provides their own ANTHROPIC_API_KEY
// Same as today, but they set the key instead of us
export ANTHROPIC_API_KEY=sk-ant-...
vibeship build "habit tracker"
```

**Pros**: Zero code changes to the model layer. Fastest execution.

**Cons**: User needs a separate API key (not their Claude Code subscription). Costs per token.

#### Option C: Hybrid (Best of Both)
- Default: Use Claude Code SDK (free for Claude Code subscribers)
- Flag: `--api-key` or `ANTHROPIC_API_KEY` env var for direct API access (faster, for power users)
- The `ModelFactory` interface stays the same — just swap the implementation

**Recommendation**: Start with **Option B** (simplest, zero model layer changes) for v1 launch. Add Claude Code SDK integration as v1.1. The value prop is "open source and free tooling" — even with their own API key, users save money vs Rork because there's no platform fee.

### 6. WebSocket + Handler → CLI Commands

**What happens today**: macOS app connects via WebSocket, sends prompt, receives AG-UI events (progress, text streaming, action requests). The handler creates channels, spawns goroutines, streams events.

**CLI replacement**: Direct function calls. No WebSocket, no channels, no goroutines for transport.

```
vibeship build "habit tracker app"
  → Parse CLI args
  → Load/create .vibeship/project.json
  → Create local ActionExecutor (direct file I/O + exec.Command)
  → Run orchestration pipeline (same graph)
  → Stream progress to terminal (spinners, colored output)
  → Write files to disk
  → Run xcodegen + xcodebuild
  → Print result
```

**Event streaming**: Replace AG-UI WebSocket events with terminal output:
```
$ vibeship build "habit tracker with streaks and notifications"

Analyzing your request...
  App: HabitTracker
  Features: Habit tracking, Streak system, Push notifications

Planning architecture...
  Files: 12 Swift files
  Models: Habit (SwiftData)
  Navigation: TabView (Habits, Stats, Settings)

Building code...
  [1/12] ContentView.swift
  [2/12] Features/Habits/HabitListView.swift
  ...

Deploying...
  Writing 12 files
  Generating project.yml
  Running xcodegen... OK
  Building project... OK (14.2s)

  HabitTracker built successfully!
  Open: open HabitTracker.xcodeproj
```

**Implementation**: New `EventEmitter` implementation that prints to terminal instead of sending to WebSocket channel.

### 7. Auth + Billing + Paywall → DELETE

Not needed for open-source CLI. Users don't authenticate. There's no subscription to check.

### 8. Redis → DELETE

Used for caching in the SaaS. CLI doesn't need a cache layer — everything is local.

### 9. Apple Docs Search → KEEP (Embedded Database)

**What it does today**: pgvector-based semantic search over crawled Apple developer documentation. The `search_apple_docs` tool lets the fixer and locator find correct SwiftUI/UIKit API usage.

**CLI approach**: Ship the crawled Apple docs as an embedded SQLite database (or just a JSON file) bundled with the CLI binary. The search becomes local keyword/fuzzy search instead of semantic search.

**Alternative**: Drop it entirely and let Claude Code's built-in web search handle Apple docs lookups. Claude Code can search developer.apple.com directly. This is simpler but slower and requires internet.

**Recommendation**: Keep a lightweight local search (JSON file + keyword matching) for v1. It's a differentiator — faster and more reliable than web search for API lookups.

---

## CLI Structure

### Command Interface

```bash
# Build a new app from a prompt
vibeship build "habit tracker with streaks and dark mode"

# Edit an existing project
vibeship edit "add push notifications"

# Fix compilation errors (reads from last build output or stdin)
vibeship fix

# Open the project in Xcode
vibeship open

# Show project info
vibeship info

# Interactive mode (multi-turn conversation)
vibeship chat
```

### Project Layout

```
~/Projects/HabitTracker/           ← user's project directory
├── .vibeship/                     ← CLI metadata (gitignored)
│   ├── project.json               ← project config (name, bundle_id)
│   ├── project_index.json         ← file descriptions, repo map, procedures
│   ├── history.json               ← conversation history + summary
│   └── last_build.log             ← last xcodebuild output (for `vibeship fix`)
├── project.yml                    ← XcodeGen spec (generated)
├── HabitTracker.xcodeproj/        ← generated by xcodegen
├── .build/                        ← xcodebuild derived data
├── HabitTracker/                  ← app source (Swift files)
│   ├── HabitTrackerApp.swift
│   ├── ContentView.swift
│   ├── Features/
│   │   ├── Habits/
│   │   │   ├── HabitListView.swift
│   │   │   └── HabitDetailView.swift
│   │   └── Settings/
│   │       └── SettingsView.swift
│   ├── Models/
│   │   └── Habit.swift
│   └── en.lproj/
│       └── Localizable.strings
└── .git/
```

### Go Package Structure

```
vibeship/
├── cmd/
│   └── vibeship/
│       └── main.go                ← CLI entry point (cobra)
├── internal/
│   ├── cli/                       ← CLI-specific code
│   │   ├── commands/              ← build, edit, fix, open, info, chat
│   │   ├── terminal/              ← terminal UI (spinners, colors, progress)
│   │   └── emitter.go             ← TerminalEventEmitter (replaces ChannelEventEmitter)
│   ├── storage/                   ← Local JSON file storage
│   │   ├── project_store.go       ← implements ProjectRepository via JSON files
│   │   ├── index_store.go         ← implements ProjectIndexRepository via JSON files
│   │   └── history_store.go       ← implements MessageRepository via JSON files
│   ├── xcode/                     ← Xcode toolchain (local exec.Command)
│   │   ├── service.go             ← implements XcodeService (write files, xcodegen, xcodebuild, git)
│   │   ├── build_parser.go        ← xcodebuild output parser
│   │   └── simulator.go           ← simulator management (future: screenshots)
│   ├── llm/                       ← LLM integration
│   │   ├── claude_api.go          ← Direct Anthropic API (user's key)
│   │   └── claude_code.go         ← Claude Code SDK subprocess (future)
│   └── modules/
│       └── ios_coder/             ← EXTRACTED from backend (the core engine)
│           ├── domain/            ← Same domain types (zero changes)
│           ├── orchestration/     ← Same graph, nodes, tools, prompts (zero changes)
│           └── app/               ← Simplified service (no Temporal, no WebSocket)
└── Makefile
```

---

## What Gets Extracted vs Rewritten vs Deleted

### EXTRACT (copy from backend, minimal changes)

| Package | Files | Changes Needed |
|---|---|---|
| `domain/` | All 21 files | Remove `ActionResultCh`/`ActionFeedbackCh` from `ProcessRequest`. Remove `ActionRequest`/`ActionFeedback` WebSocket wire types. Keep everything else. |
| `orchestration/` | All 68+ files | **Zero changes**. All nodes, tools, prompts, constraints, graph — identical. This is the entire value. |
| `infra/treesitter/` | 4 files | Zero changes. Runs locally, reads files from disk. |
| `domain/xcodegen.go` | 1 file | Zero changes. Generates `project.yml` YAML. |

### REWRITE (new implementations of existing interfaces)

| Component | Old Implementation | New Implementation |
|---|---|---|
| `ProjectRepository` | PostgreSQL via SQLC | `storage/project_store.go` — JSON file I/O |
| `ProjectIndexRepository` | PostgreSQL via SQLC | `storage/index_store.go` — JSON file I/O |
| `MessageRepository` | PostgreSQL via SQLC | `storage/history_store.go` — JSON file I/O |
| `ActionExecutor` | `WSActionExecutor` (WebSocket RPC) | `xcode/service.go` — local `exec.Command` |
| `EventEmitter` | `ChannelEventEmitter` (WebSocket) | `cli/emitter.go` — terminal output (spinners, progress) |
| `ModelFactory` | `claude.NewChatModel` (direct API) | Same, but with user's API key. Future: Claude Code SDK. |
| Service layer | `app/service.go` (Temporal, DB messages) | Simplified: no Temporal, local file storage |

### DELETE (not needed in CLI)

| Component | Reason |
|---|---|
| `handler.go` | WebSocket handler — CLI calls service directly |
| `routes.go` | HTTP routes — no server |
| `provider.go` | DI container (uber-go/dig) — CLI uses direct construction |
| `app/action_executor.go` | `WSActionExecutor` — replaced by local executor |
| `app/heartbeat_emitter.go` | WebSocket liveness — no WebSocket |
| `app/summary_workflow.go` | Temporal workflow — runs synchronously |
| `app/repomap_workflow.go` | Temporal workflow — runs synchronously |
| `app/procedures_workflow.go` | Temporal workflow — runs synchronously |
| `infra/lancedb/` | Vector store — dropped for v1 |
| `infra/voyage/` | Embedding service — dropped for v1 |
| `infra/repositories/project_index_repository.go` | PostgreSQL implementation — replaced by local JSON |
| All auth, billing, paywall, users, projects, chats, workspace modules | SaaS features |
| All PostgreSQL, Redis, Temporal infrastructure | Server infrastructure |

---

## Post-Build Pipeline (Replaces Temporal Workflows)

After each successful build or edit, run these synchronously:

```go
func (s *service) postBuildTasks(ctx context.Context, projectDir string, indexData *domain.ProjectIndexData) {
    // 1. Generate repo map (tree-sitter) — ~1-2 seconds
    repoMap, err := s.repoMapBuilder.Build(ctx, filepath.Join(projectDir, appName))
    if err == nil {
        indexData.RepoMap = repoMap
    }

    // 2. Generate procedures (LLM call) — ~3-5 seconds
    procedures, err := s.proceduresGenerator.Generate(ctx, indexData)
    if err == nil {
        indexData.Procedures = procedures
    }

    // 3. Generate conversation summary (LLM call) — ~2-3 seconds
    summary, err := s.summaryGenerator.Generate(ctx, history)
    if err == nil {
        s.store.SaveConversationSummary(summary)
    }

    // 4. Save index to disk
    s.store.SaveProjectIndex(indexData)
}
```

Total: ~6-10 seconds after build. Shown as a terminal spinner: "Updating project index..."

---

## LLM Integration Strategy

### Phase 1 (Launch): Direct Anthropic API

Users provide their own API key:
```bash
export ANTHROPIC_API_KEY=sk-ant-...
vibeship build "habit tracker"
```

**Why**: Zero changes to the model layer. The `ModelFactory` pattern works identically. Ship fast.

**Cost for users**: A typical build uses ~$0.30-0.80 in API calls (Sonnet for builder, Haiku for everything else). Much cheaper than Rork's $200.

### Phase 2: Claude Code SDK Integration

Invoke Claude Code as subprocess for LLM calls:
```go
type ClaudeCodeModelFactory struct{}

func (f *ClaudeCodeModelFactory) Create(modelName string) model.ToolCallingChatModel {
    return &ClaudeCodeModel{modelName: modelName}
}

type ClaudeCodeModel struct {
    modelName string
}

func (m *ClaudeCodeModel) Generate(ctx context.Context, messages []*schema.Message, opts ...model.Option) (*schema.Message, error) {
    // Build claude CLI command
    // claude --print --model <modelName> --system-prompt <system> --max-tokens <max>
    // Pipe messages as JSON via stdin
    // Parse JSON response
    // Return as schema.Message
}
```

**Benefit**: Users with Claude Code Max ($100/mo) or Team ($200/mo) get unlimited builds at no per-token cost. This is the killer feature vs Rork.

**Challenge**: Claude Code subprocess adds ~200-500ms latency per call. A build has ~8-15 LLM calls. Total overhead: ~2-8 seconds. Acceptable.

### Phase 3: Claude Code as MCP Server

Register vibeship as an MCP tool that Claude Code can invoke:
```json
{
  "mcpServers": {
    "vibeship": {
      "command": "vibeship",
      "args": ["mcp-serve"]
    }
  }
}
```

Then users just ask Claude Code: "Build me a habit tracker iOS app" and it invokes vibeship tools automatically.

---

## Distribution

### Install Methods

```bash
# Homebrew (macOS — primary)
brew install vibeship/tap/vibeship

# Go install
go install github.com/vibeship/vibeship@latest

# Download binary
curl -fsSL https://vibeship.dev/install.sh | sh
```

### Prerequisites

```bash
# Required
xcode-select --install     # Xcode Command Line Tools
brew install xcodegen       # XcodeGen for project generation

# LLM access (one of):
export ANTHROPIC_API_KEY=sk-ant-...   # Direct API key
# OR: Claude Code subscription (Phase 2)
```

### First Run

```bash
mkdir my-app && cd my-app
vibeship build "habit tracker with streaks, dark mode, and push notifications"
```

---

## Monetization (Paid Add-Ons)

The CLI is 100% free and open source. Revenue from optional cloud services:

| Feature | What It Does | Why Paid |
|---|---|---|
| **Image Generation** | AI-generated app icons, splash screens, marketing assets | Requires DALL-E/Midjourney API calls |
| **Lottie Animations** | AI-generated animations for onboarding, empty states | Requires specialized generation |
| **Cloud Build** | Build on server for users without a Mac | Requires Mac infrastructure |
| **App Store Screenshots** | Auto-generate marketing screenshots with device frames | Requires simulator + rendering pipeline |
| **Premium Templates** | Curated, high-quality app templates (e.g., social media, marketplace) | Curation effort |
| **Priority Support** | Direct access to maintainers | Time cost |

These are separate services with their own API. The CLI calls them when available:
```bash
vibeship build "habit tracker" --generate-icon --generate-screenshots
# → Calls paid API for icon/screenshots, everything else is free
```

---

## Implementation Order

### Week 1: Core CLI + Local Storage
1. Set up Go CLI with cobra (cmd/vibeship/main.go)
2. Implement `storage/` package (project, index, history as JSON files)
3. Implement `xcode/service.go` (local ActionExecutor: write files, xcodegen, xcodebuild, git)
4. Implement `cli/emitter.go` (terminal output with spinners and progress)
5. Copy `domain/` and `orchestration/` from backend (zero changes needed)
6. Wire it all together: `vibeship build` command end-to-end

### Week 2: Polish + Ship
7. Implement `vibeship edit` and `vibeship fix` commands
8. Add repo map generation (synchronous, post-build)
9. Add procedures generation (synchronous, post-build)
10. Add conversation history + summary (for multi-turn context)
11. Test end-to-end: build → edit → fix cycle
12. README, demo video, `brew tap` setup
13. Ship to GitHub, post on Twitter/Reddit while Rork is trending

### Week 3+: Claude Code Integration
14. Add Claude Code SDK subprocess integration (Phase 2)
15. Add MCP server mode (Phase 3)
16. Paid add-ons API scaffold

---

## Key Risk: Build Speed

A full `vibeship build` involves:
- ~8-15 LLM calls (router, analyzer, planner, builder, deployer Haiku calls)
- ~30-60 seconds for LLM generation
- ~5-15 seconds for xcodebuild
- ~5-10 seconds for post-build tasks (repo map, procedures, summary)
- **Total: ~1-2 minutes** for a fresh build

This is comparable to Rork. Edit flows are faster (~20-30 seconds) because they only regenerate changed files.

If using Claude Code SDK (Phase 2), add ~2-8 seconds subprocess overhead. Still acceptable.

---

## Key Risk: Orchestration Quality

The orchestration engine IS the product. If generated apps have bugs, missing settings wiring, broken localization — users won't come back regardless of the price.

**Advantage**: The backend's orchestration layer has been heavily refined (constraints.go, feature rules, settings wiring fixes, localization bug fixes, notification permission handling). All of that knowledge transfers to the CLI — it's the same code.

**The prompts, the graph, the tools, the constraints — that's the moat.** Open-sourcing the CLI doesn't give away the moat because:
1. The prompts encode months of debugging SwiftUI edge cases
2. The graph structure (analyzer → planner → builder → deployer → fixer loop) is battle-tested
3. Someone can fork it but can't easily improve it without the same iteration cycles

---

## Summary

| Aspect | SaaS (Current) | CLI (New) |
|---|---|---|
| Infrastructure | PostgreSQL + Temporal + Redis + LanceDB | JSON files on disk |
| LLM Access | Our Anthropic API key (we pay) | User's API key or Claude Code sub (they pay) |
| Build Execution | WebSocket RPC to macOS client | Local exec.Command on user's Mac |
| Auth/Billing | Google/Apple Sign-In + Polar.sh | None (open source) |
| Distribution | App Store (macOS app) | brew install / go install |
| Revenue | Subscription ($X/mo) | Paid add-ons (image gen, cloud build, etc.) |
| Orchestration Engine | Identical | Identical |
| Code Changes | — | ~20% rewrite (storage, executor, emitter), 80% extract as-is |
