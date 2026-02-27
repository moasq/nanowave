# Milestone-Based Generation Pipeline

## Problem Statement

### The Evidence

The current pipeline sends **all planned files to the builder in a single pass**. For Pulsers (35 files, ~1,384 LOC), the build prompt contains every file description, every model, every integration instruction, every skill rule, and every design token simultaneously. Research shows this is the primary failure mode:

- **NoLiMa benchmark**: 11/12 models drop below 50% baseline at 32K tokens
- **Stanford "Lost in the Middle"**: LLMs lose 13.9-85% accuracy on information buried in mid-context
- **Spotify (1,500+ agent PRs)**: enforces "single change per prompt" as a hard rule
- **Self-Planning Code Generation (ACM)**: multi-stage yields 25.4% improvement in Pass@1
- **Multi-Stage Guided Generation (2025)**: 34.7% relative improvement over direct generation
- **Addy Osmani ("The 80% Problem")**: "misunderstanding early on cascades across the entire feature"

### What We Observe

| Symptom | Root Cause |
|---------|-----------|
| Repository protocol files are 6-9 lines (barely worth existing) | Planner generates maximum architecture regardless of complexity |
| Auth + Supabase + Realtime + UI all compete for attention | Single pass forces the model to hold everything at once |
| Completion gate needs 2-3 passes to catch missed files | Files generated late in the session suffer from context fatigue |
| ViewModels sometimes use `.task` instead of `init()` loading | Rules at prompt start get diluted by the time views are generated |

### The Insight

The planner already produces a `build_order` with dependency information. We are one step away from grouping those files into **milestones** — self-contained build phases that each compile independently and build on the previous milestone's output.

---

## Proposed Architecture

### Overview

```
Current:  Plan → Build(all 35 files) → Completion(all) → Fix → Done

Proposed: Plan(with milestones) → for each milestone:
            Build(milestone files) → Verify(milestone) → Compile
          → Done
```

### Milestone Categories

The planner assigns each file to exactly one milestone. These are the canonical milestone types:

| # | Milestone | Scope | Always Present |
|---|-----------|-------|----------------|
| 1 | `foundation` | Models, Theme, Config, Shared types, App entry | Yes |
| 2 | `features` | Feature views + ViewModels (in-memory sampleData) | Yes |
| 3 | `auth` | AuthService, AuthView, AuthGuard, RootView wiring | Only if auth needed |
| 4 | `data` | Repositories, backend service, ViewModel async migration | Only if backend integration |
| 5 | `polish` | Extensions, advanced UX states, realtime, uploads | Only if extensions/realtime |

### What Each Milestone Produces

**Milestone 1 — Foundation** (~8-10 files)
```
Models/*.swift          (with static sampleData)
Theme/AppTheme.swift
Config/AppConfig.swift  (if backend integration planned)
Shared/Loadable.swift   (if async data planned)
App/PulsersApp.swift
App/RootView.swift
App/MainView.swift
```
- **Exit criteria**: compiles, app launches to MainView with tab structure
- **Context load**: low — only structural decisions, no business logic
- **What the model focuses on**: data shapes, design tokens, navigation skeleton

**Milestone 2 — Features** (~10-15 files)
```
Features/Rooms/RoomsListView.swift
Features/Rooms/RoomsListViewModel.swift
Features/Rooms/RoomRowView.swift
Features/Rooms/CreateRoomView.swift
Features/LiveRoom/LiveRoomView.swift
Features/LiveRoom/LiveRoomViewModel.swift
Features/LiveRoom/ParticipantCardView.swift
Features/LiveRoom/RepTapButton.swift
Features/Profile/ProfileView.swift
Features/Profile/ProfileViewModel.swift
Features/Profile/WorkoutHistoryRowView.swift
Features/Common/AuthGuardView.swift  (placeholder if auth comes later)
```
- **Exit criteria**: compiles, all screens render with sampleData, navigation works
- **Context load**: medium — feature logic, but no auth/backend complexity
- **What the model focuses on**: UI composition, view-model wiring, design system compliance
- **Key**: ViewModels use `Model.sampleData` directly. No async. No Loadable yet.

**Milestone 3 — Auth** (~3-5 files, modify 2-3)
```
NEW:    Services/Auth/AuthService.swift
NEW:    Features/Auth/AuthView.swift
NEW:    Features/Auth/AuthViewModel.swift
MODIFY: App/RootView.swift          (wrap in AuthGuardView)
MODIFY: Features/Common/AuthGuardView.swift  (real implementation)
```
- **Exit criteria**: compiles, auth flow works (sign in → guarded content → sign out)
- **Context load**: low — only auth concern. Existing app is the stable foundation.
- **What the model focuses on**: Supabase auth APIs, Apple Sign-In, session management

**Milestone 4 — Data Layer** (~8-10 files, modify 4-6)
```
NEW:    Services/Supabase/SupabaseService.swift
NEW:    Repositories/Rooms/WorkoutRoomRepository.swift (protocol)
NEW:    Repositories/Rooms/SupabaseWorkoutRoomRepository.swift
NEW:    Repositories/Profile/ProfileRepository.swift (protocol)
NEW:    Repositories/Profile/SupabaseProfileRepository.swift
NEW:    Repositories/Participants/RoomParticipantRepository.swift (protocol)
NEW:    Repositories/Participants/SupabaseRoomParticipantRepository.swift
NEW:    Repositories/Sessions/WorkoutSessionRepository.swift (protocol)
NEW:    Repositories/Sessions/SupabaseWorkoutSessionRepository.swift
MODIFY: Features/Rooms/RoomsListViewModel.swift     (sampleData → Loadable + repo)
MODIFY: Features/Profile/ProfileViewModel.swift      (sampleData → Loadable + repo)
MODIFY: Features/LiveRoom/LiveRoomViewModel.swift    (sampleData → Loadable + repo + realtime)
```
- **Exit criteria**: compiles, data loads from Supabase, Loadable states handled
- **Context load**: medium — data layer only. UI already works. Auth already works.
- **What the model focuses on**: Supabase queries, DTOs, repository pattern, Loadable migration

**Milestone 5 — Polish** (~2-5 files, modify 2-3)
```
NEW:    Targets/PulsersWidget/*.swift  (if widget extension planned)
NEW:    Shared/ActivityAttributes.swift (if live activity planned)
MODIFY: Views with Loadable states    (ensure all 4 states handled)
MODIFY: Views with mutation buttons   (disable + spinner pattern)
```
- **Exit criteria**: compiles, all UX states present, extensions work
- **Context load**: low — focused polish pass

### Adaptive Complexity: Small Apps Skip Milestones

Not every app needs 5 milestones. The planner decides based on complexity:

| App Type | Example | Milestones |
|----------|---------|-----------|
| Simple in-memory | Timer, Calculator, Notes | `foundation` + `features` (2 milestones) |
| Local + auth | Habit tracker with Sign-In | `foundation` + `features` + `auth` (3 milestones) |
| Full backend | Pulsers, social app | All 5 milestones |
| Backend, no auth | Public feed reader | `foundation` + `features` + `data` (3 milestones, skip auth) |

**Threshold rule**: If total planned files ≤ 18, the planner MAY collapse into 2 milestones (`foundation` + `features`). The pipeline respects whatever the planner assigns.

---

## Implementation Plan

### Phase 1: Type Changes (`types.go`)

Add milestone field to `FilePlan` and milestone metadata to `PlannerResult`:

```go
type FilePlan struct {
    Path       string   `json:"path"`
    TypeName   string   `json:"type_name"`
    Purpose    string   `json:"purpose"`
    Platform   string   `json:"platform,omitempty"`
    Components string   `json:"components"`
    DataAccess string   `json:"data_access"`
    DependsOn  []string `json:"depends_on"`
    Milestone  string   `json:"milestone"`          // NEW: "foundation", "features", "auth", "data", "polish"
}

// MilestoneOrder defines the canonical execution sequence.
var MilestoneOrder = []string{"foundation", "features", "auth", "data", "polish"}
```

Add helper to `PlannerResult`:

```go
// Milestones returns the ordered list of unique milestones present in the plan.
func (p *PlannerResult) Milestones() []string {
    seen := make(map[string]bool)
    for _, f := range p.Files {
        seen[f.Milestone] = true
    }
    var result []string
    for _, m := range MilestoneOrder {
        if seen[m] {
            result = append(result, m)
        }
    }
    return result
}

// FilesForMilestone returns files assigned to the given milestone, in build order.
func (p *PlannerResult) FilesForMilestone(milestone string) []FilePlan {
    orderIndex := make(map[string]int)
    for i, path := range p.BuildOrder {
        orderIndex[path] = i
    }
    var files []FilePlan
    for _, f := range p.Files {
        if f.Milestone == milestone {
            files = append(files, f)
        }
    }
    sort.Slice(files, func(i, j int) bool {
        return orderIndex[files[i].Path] < orderIndex[files[j].Path]
    })
    return files
}
```

### Phase 2: Planner Prompt Changes

Update `skills/phases/planner/references/output-format.md` to require milestone assignment:

```
Each file entry MUST include a "milestone" field:
- "foundation": Models, Theme, Config, Shared types, App entry point, RootView, MainView
- "features": Feature views + ViewModels using in-memory sampleData
- "auth": Auth service, auth views, auth guard (only if auth is needed)
- "data": Repositories, backend services, ViewModel async migration (only if backend integration)
- "polish": Extensions, advanced UX, realtime subscriptions, upload flows

Rules:
- Every file gets exactly one milestone
- foundation files have NO dependencies on features/auth/data files
- features files depend only on foundation files (use sampleData, not repositories)
- auth files depend on foundation (and optionally modify features files)
- data files depend on foundation + features (migrate ViewModels from sampleData to Loadable)
- polish files depend on everything above
- If total files ≤ 18 and no backend integration: collapse to "foundation" + "features" only
```

Update `skills/phases/planner/references/workflow.md` validation checklist:

```
16. Every file has a milestone assignment from: foundation, features, auth, data, polish.
17. foundation files have zero depends_on references to features/auth/data/polish files.
18. features ViewModels use sampleData (not Loadable) — async migration happens in data milestone.
```

### Phase 3: Pipeline Changes (`pipeline.go`)

Replace the single build loop with a milestone loop. The key change is in Phase 4:

```go
// Phase 4: Milestone-based build
milestones := plan.Milestones()
terminal.Info(fmt.Sprintf("Building in %d milestones: %s", len(milestones), strings.Join(milestones, " → ")))

var (
    sessionID         string
    totalCostUSD      float64
    totalInputTokens  int
    totalOutputTokens int
    totalCacheRead    int
    totalCacheCreate  int
    totalPasses       int
)

for mi, milestone := range milestones {
    msFiles := plan.FilesForMilestone(milestone)
    progress := terminal.NewProgressDisplay(milestone, len(msFiles))
    progress.Start()

    // Build this milestone
    for pass := 1; pass <= maxMilestoneCompletionPasses; pass++ {
        var resp *claude.Response
        var err error

        if pass == 1 {
            resp, err = p.buildMilestoneStreaming(ctx, prompt, appName, projectDir,
                analysis, plan, milestone, msFiles, sessionID, progress,
                images, activeIntegrationIDs, backendProvisioned)
        } else {
            msReport := verifyMilestoneFiles(projectDir, appName, msFiles, plan)
            resp, err = p.completeMilestoneMissingStreaming(ctx, appName, projectDir,
                plan, milestone, msReport, sessionID, progress)
        }
        if err != nil {
            progress.StopWithError(fmt.Sprintf("%s pass %d failed", milestone, pass))
            return nil, fmt.Errorf("%s build failed: %w", milestone, err)
        }

        // Accumulate usage
        totalCostUSD += resp.TotalCostUSD
        totalInputTokens += resp.Usage.InputTokens
        totalOutputTokens += resp.Usage.OutputTokens
        totalCacheRead += resp.Usage.CacheReadInputTokens
        totalCacheCreate += resp.Usage.CacheCreationInputTokens
        totalPasses++
        if resp.SessionID != "" {
            sessionID = resp.SessionID
        }

        // Verify this milestone's files
        msReport := verifyMilestoneFiles(projectDir, appName, msFiles, plan)
        if msReport.Complete {
            break
        }
        if pass >= maxMilestoneCompletionPasses {
            progress.StopWithError(fmt.Sprintf("%s incomplete after %d passes", milestone, pass))
            // Continue to next milestone — don't fail the whole build
            // Completion recovery will catch stragglers
            break
        }
    }

    progress.StopWithSuccess(fmt.Sprintf("%s complete (%d files)", milestone, len(msFiles)))
}

// Final verification: check ALL files across all milestones
report := verifyPlannedFiles(projectDir, appName, plan)
if !report.Complete {
    // One final recovery pass for any stragglers
    // ...existing completeMissingFilesStreaming logic...
}
```

### Phase 4: Build Prompt Changes (`build_prompts.go`)

New function `buildMilestonePrompts()` that constructs a focused prompt for a single milestone:

```go
func (p *Pipeline) buildMilestonePrompts(
    prompt, appName, projectDir string,
    analysis *AnalysisResult, plan *PlannerResult,
    milestone string, msFiles []FilePlan,
    backendProvisioned bool,
) (string, string, error) {
    // System append prompt: same coder base + rules
    basePrompt, err := composeCoderAppendPrompt("builder", plan.GetPlatform())

    var appendPrompt strings.Builder
    appendPrompt.WriteString(basePrompt)

    // Milestone context header
    fmt.Fprintf(&appendPrompt, "\n\n<milestone phase=%q>\n", milestone)
    appendPrompt.WriteString(milestoneInstructions(milestone))

    // Design (always needed)
    appendPrompt.WriteString("\n## Design\n")
    // ...same design section as current...

    // Models (always needed — data shapes are universal)
    appendPrompt.WriteString("\n### Models\n")
    // ...same model section as current...

    // Files — ONLY THIS MILESTONE'S FILES
    appendPrompt.WriteString("\n## Files (build in this order)\n")
    for _, f := range msFiles {
        appendBuildPlanFileEntry(&appendPrompt, f)
    }

    appendPrompt.WriteString("\n</milestone>\n")

    // Feature rules — only include rules relevant to this milestone's files
    relevantRules := milestoneRelevantRules(milestone, plan.RuleKeys)
    if len(relevantRules) > 0 {
        appendPrompt.WriteString("\n<feature-rules>\n")
        for _, key := range relevantRules {
            content := loadRuleContent(key)
            if content != "" {
                appendPrompt.WriteString("\n")
                appendPrompt.WriteString(content)
                appendPrompt.WriteString("\n")
            }
        }
        appendPrompt.WriteString("</feature-rules>\n")
    }

    // Integration config — only for "data" and "auth" milestones
    if (milestone == "auth" || milestone == "data") && len(plan.Integrations) > 0 {
        appendIntegrationConfig(&appendPrompt, ...)
    }

    // User message — milestone-specific
    userMsg := buildMilestoneUserMessage(milestone, msFiles, appName, analysis, plan, ...)

    return appendPrompt.String(), userMsg, nil
}
```

#### Milestone-Specific Instructions

```go
func milestoneInstructions(milestone string) string {
    switch milestone {
    case "foundation":
        return `You are building the FOUNDATION layer.
Write: Models (with sampleData), Theme, Config, Shared types, and App entry point.
After this milestone the app MUST compile and launch to an empty MainView shell.
Do NOT write feature views or ViewModels yet — those come in the next milestone.
Do NOT add any async/Loadable logic yet — foundation uses only static data.`

    case "features":
        return `You are building the FEATURES layer on top of an existing foundation.
The Models, Theme, and App entry point already exist — read them first.
Write: Feature Views + ViewModels using IN-MEMORY sampleData.
ViewModels should initialize data from Model.sampleData — NOT from repositories or async calls.
After this milestone the app MUST compile and show working UI with sample data on every screen.
Do NOT add authentication, Supabase calls, or Loadable<T> patterns yet.`

    case "auth":
        return `You are adding AUTHENTICATION to an existing working app.
Read the existing app structure first — Models, Views, and ViewModels already work.
Write: AuthService, AuthView, AuthViewModel, and modify RootView to add AuthGuardView.
After this milestone the app MUST compile with working sign-in → guarded content → sign-out flow.
Do NOT modify ViewModels to use repositories yet — that comes in the data milestone.`

    case "data":
        return `You are adding the DATA LAYER to an existing working app with auth.
Read existing ViewModels first — they currently use sampleData.
Write: SupabaseService, Repository protocols + implementations.
MODIFY existing ViewModels: replace sampleData with Loadable<T> + repository calls.
Add init() { Task { await load() } } to each ViewModel for first-load.
After this milestone the app MUST compile and load real data from Supabase.
Ensure every view handles all 4 Loadable states (loading, empty, data, error).`

    case "polish":
        return `You are POLISHING an existing working app with auth and live data.
Read the existing app structure first — everything already works.
Add: Extensions (widgets, live activities), advanced UX states, realtime subscriptions.
Ensure every mutation button is disabled while in-progress with an inline spinner.
After this milestone the app MUST compile with all planned features complete.`
    }
    return ""
}
```

### Phase 5: Completion Changes (`completion.go`)

Add milestone-scoped verification:

```go
// verifyMilestoneFiles checks only the files belonging to a specific milestone.
func verifyMilestoneFiles(projectDir, appName string, msFiles []FilePlan, plan *PlannerResult) *FileCompletionReport {
    // Same logic as verifyPlannedFiles but scoped to msFiles
    report := &FileCompletionReport{TotalPlanned: len(msFiles)}
    for _, f := range msFiles {
        status := checkPlannedFile(projectDir, appName, f, plan)
        if status.Valid {
            report.ValidCount++
        } else if !status.Exists {
            report.Missing = append(report.Missing, status)
        } else {
            report.Invalid = append(report.Invalid, status)
        }
    }
    report.Complete = report.ValidCount == report.TotalPlanned
    return report
}
```

### Phase 6: Backward Compatibility

The `milestone` field is optional in the JSON contract. When the planner omits it (or for existing plans), the pipeline falls back to the current single-pass behavior:

```go
milestones := plan.Milestones()
if len(milestones) == 0 {
    // Legacy path: no milestones assigned, run single-pass build
    return p.buildLegacy(ctx, ...)
}
```

This allows gradual rollout:
1. Ship the type changes and pipeline support first
2. Update planner prompt to assign milestones
3. Test with milestone-aware plans
4. Remove legacy path once stable

---

## Session Management

### Option A: Fresh Session Per Milestone (Recommended)

Each milestone starts a fresh Claude session. Benefits:
- **Clean context** — no accumulated noise from previous milestones
- **Models, Theme, and existing files are read from disk** — the agent reads CLAUDE.md and existing source
- **Session ID returned per milestone** for Edit/Fix targeting

```
Milestone 1: session_A → foundation files written to disk
Milestone 2: session_B → reads disk, writes features (clean context)
Milestone 3: session_C → reads disk, writes auth (clean context)
Milestone 4: session_D → reads disk, writes data layer (clean context)
```

### Option B: Continued Session

Reuse session across milestones. Benefits:
- Cache hits reduce cost
- Agent remembers decisions from previous milestones

Drawbacks:
- Context grows with each milestone (the problem we're solving)
- Errors from milestone 1 pollute milestone 3

**Recommendation**: Option A. The agent reads existing files from disk at the start of each milestone — this provides full context without polluting the generation window. CLAUDE.md + memory files persist architectural decisions across sessions.

---

## Prompt Token Budget Analysis

### Current (Single Pass — Pulsers)

| Component | Est. Tokens |
|-----------|------------|
| Coder base + constraints | ~2,000 |
| Build plan (35 files × ~80 tokens) | ~2,800 |
| Models (4 models × ~100 tokens) | ~400 |
| Design section | ~200 |
| Feature rules (5-8 skills × ~500 tokens) | ~3,000 |
| Integration config (Supabase) | ~1,500 |
| Backend-first instructions | ~300 |
| User message | ~500 |
| **Total system+user** | **~10,700** |

Plus the agent reads CLAUDE.md (~2K), skills (~5K), and project files as it works. Effective context during generation: **25-40K tokens**.

### Proposed (Per Milestone — Pulsers)

| Milestone | Files | Plan Tokens | Rules Tokens | Integration | Total |
|-----------|-------|-------------|-------------|-------------|-------|
| foundation | 8 | ~640 | ~1,000 | 0 | ~4,500 |
| features | 12 | ~960 | ~2,000 | 0 | ~5,600 |
| auth | 5 | ~400 | ~500 | ~500 | ~4,000 |
| data | 10 | ~800 | ~500 | ~1,500 | ~5,400 |
| polish | 3 | ~240 | ~500 | 0 | ~3,400 |

**Peak context per milestone: ~5,600 tokens** (vs ~10,700 current). Effective generation context drops from ~35K to ~15-20K per milestone.

### Cost Impact

More API calls (5 milestones × 1-2 passes vs 1-6 passes), but:
- **Cache hits on CLAUDE.md and skills** amortize across milestones
- **Fewer completion recovery passes** (focused context → fewer missed files)
- **Less wasted generation** (no files rewritten because of context confusion)

Estimated net cost change: **roughly neutral** (±15%). More calls, but smaller and more efficient.

---

## UX Changes

### Terminal Output

```
⠋ Setting up workspace...
✓ Workspace ready
⠋ Analyzing...
✓ Analyzed: Pulsers
⠋ Planning...
✓ Plan ready (35 files, 4 models, 5 milestones)

Building in 5 milestones: foundation → features → auth → data → polish

⠋ foundation (8 files)
  ✓ Models/WorkoutRoom.swift
  ✓ Models/Profile.swift
  ...
✓ foundation complete (8/8 files, compiled)

⠋ features (12 files)
  ✓ Features/Rooms/RoomsListView.swift
  ✓ Features/Rooms/RoomsListViewModel.swift
  ...
✓ features complete (12/12 files, compiled)

⠋ auth (5 files)
  ✓ Services/Auth/AuthService.swift
  ...
✓ auth complete (5/5 files, compiled)

⠋ data (8 files)
  ✓ Repositories/Rooms/WorkoutRoomRepository.swift
  ...
✓ data complete (8/8 files, compiled)

⠋ polish (2 files)
  ...
✓ polish complete (2/2 files, compiled)

✓ Build complete — 35 files, 5 milestones, $0.42
```

### Build Result

```go
type BuildResult struct {
    // ...existing fields...
    Milestones       []MilestoneResult  // NEW
}

type MilestoneResult struct {
    Name            string
    FilesPlanned    int
    FilesCompleted  int
    Passes          int
    CostUSD         float64
}
```

---

## Rollout Plan

### Step 1: Types + Planner (Low Risk)
- Add `Milestone` field to `FilePlan` in `types.go`
- Add `Milestones()` and `FilesForMilestone()` helpers
- Update planner prompt to assign milestones
- **No pipeline changes** — milestone field is ignored by current builder
- Ship, test planner output, validate milestone assignments

### Step 2: Pipeline + Prompts (Medium Risk)
- Add `buildMilestonePrompts()` to `build_prompts.go`
- Add milestone loop to `pipeline.go`
- Add `verifyMilestoneFiles()` to `completion.go`
- Keep legacy fallback for plans without milestones
- Ship behind a flag or detect from plan (no milestone field → legacy)

### Step 3: Tune + Remove Legacy (Low Risk)
- Monitor milestone pass counts, cost, quality
- Adjust milestone instructions based on failure patterns
- Remove legacy single-pass path once milestone generation is stable
- Tune `maxMilestoneCompletionPasses` (likely 3, not 6)

---

## Open Questions

1. **Should the data milestone provision the Supabase backend, or should that stay in the pipeline before any milestones?**
   Current: pipeline provisions tables before build. Proposed: keep this — backend provisioning is not code generation, it's infrastructure.

2. **Should Edit/Fix be milestone-aware?**
   Probably not initially. Edit/Fix operate on an existing project and are already scoped to user intent. The milestone structure only matters during initial Build.

3. **Can the planner collapse milestones for small apps?**
   Yes. If ≤ 18 files and no backend, planner assigns only `foundation` + `features`. The pipeline loop handles any number of milestones generically.

4. **What if a milestone fails to compile?**
   The pipeline should attempt one fix pass within the milestone before moving on. If it still fails, stop the build — downstream milestones depend on compilation. This is different from the current approach where completion failures allow retry but compilation failures are not explicitly gated between file groups.

5. **Should we use the same session or fresh sessions between milestones?**
   Start with fresh sessions (Option A). If cost is a concern, experiment with continued sessions later. Fresh sessions are safer and align with the research on context discipline.
