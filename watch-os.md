# watchOS Platform Support Plan

> Feature-safe, research-backed, full build/edit/fix parity

---

## Summary

Add first-class watchOS support to the generator with a strong emphasis on safe feature selection and clear failure behavior for unsupported requests.

This plan implements:

- **watchOS as a real platform** (not overloaded into `device_family`)
- **Both watch project shapes:**
  - Watch app only (standalone)
  - iPhone app + Watch app (standard wiring only, no auto-sync scaffolding)
- **Full parity across:** generation, edit/fix build prompts, generated project scaffold/docs/commands, XcodeGen MCP config tooling
- **Feature compatibility gate** that rejects unsupported or unverified feature selections early with explicit reasons
- **watchOS asset catalog variant** with watch-specific icon sizes and complication image sets
- **Modern single-target watch app structure** (Xcode 14+, no legacy WatchKit Extension split)

This plan defers tvOS (as requested).

---

## Research Basis

### Key Findings

| Source | Finding |
|--------|---------|
| [Apple: Creating a watchOS project](https://developer.apple.com/documentation/watchos-apps/creating-a-watchos-project) | Xcode 14+ uses single-target watch apps (old WatchKit App + Extension merged). Templates: *Watch App* (standalone) and *iOS App with Watch App* (paired). |
| [Apple: Independent watchOS apps](https://developer.apple.com/documentation/watchos-apps/creating-independent-watchos-apps) | Two independent modes: `WKWatchOnly` (no iOS companion at all) vs `WKRunsIndependentlyOfCompanionApp` (iOS companion exists but watch works alone). Watch apps request permissions directly. |
| [Apple: watchOS apps overview](https://developer.apple.com/watchos-apps/) | Widgets/complications via WidgetKit; Live Activities on Apple Watch are mirrored from paired iPhone only (no watch-native ActivityKit). |
| [Apple: WidgetKit planning](https://developer.apple.com/documentation/widgetkit/planning-your-widgets-and-live-activities-for-apple-platforms) | Watch complications use WidgetKit families: `.accessoryCircular`, `.accessoryRectangular`, `.accessoryInline`, `.accessoryCorner` (watchOS-exclusive). Extension point: `com.apple.widgetkit-extension`. |
| [WWDC25: Foundation Models](https://developer.apple.com/videos/play/wwdc2025/301/) | Foundation Models targets iOS 26+/iPadOS 26+/macOS 26+ only. Not available on watchOS. |
| [XcodeGen ProjectSpec](https://github.com/yonaskolb/XcodeGen/blob/master/Docs/ProjectSpec.md) | `supportedDestinations` explicitly prohibited for watchOS app targets (validation rejects it). Must use `platform: watchOS`. Product types: `application.watchapp2-container` (standalone), `application.watchapp2` (paired). |
| [Apple: TN3157](https://developer.apple.com/documentation/technotes/tn3157-updating-your-watchos-project-for-swiftui-and-widgetkit) | Authoritative guide for modern single-target watch app structure. |
| [Apple: Version unification (WWDC25)](https://developer.apple.com/watchos/) | As of 2025, all Apple platforms unified to version 26 (iOS 26, watchOS 26, macOS 26, etc.). Previous independent numbering (iOS 18 = watchOS 11) is superseded. |

### Verified Framework Availability on watchOS

| Framework | watchOS? | Min Version | Notes |
|-----------|----------|-------------|-------|
| MapKit | **Yes** | 2.0 | Full SwiftUI `Map` since watchOS 7. Major expansion in watchOS 26 (directions, search, overlays). No overlay support pre-watchOS 26. |
| Swift Charts | **Yes** | 9.0 | Fully available. Same `Chart` API as iOS. |
| HealthKit | **Yes** | 2.0 | Primary watch platform. `HKWorkoutSession`, activity rings, direct sensor access. |
| UserNotifications | **Yes** | 3.0 | Local and remote. Independent apps can schedule without iPhone. |
| SiriKit / App Intents | **Yes** | 3.2 / 9.0 | App Intents (modern) preferred over legacy SiriKit domains. |
| LocalAuthentication | **Yes** | 9.0 | Uses **wrist detection** (not Face ID/Touch ID). `LAPolicy.deviceOwnerAuthenticationWithWristDetection`. |
| WatchConnectivity | **Yes** | 2.0 | Exists but deliberately not scaffolded per plan. |
| AVFoundation | **Partial** | 3.0 | Audio only. No camera/capture APIs (no camera hardware). |
| Speech | **No** | — | Not available on watchOS at all. |
| StoreKit RequestReviewAction | **No** | — | Not available on watchOS. In-app purchases work, but review prompts do not. |
| Foundation Models | **No** | — | iOS 26+/iPadOS 26+/macOS 26+ only. |
| CoreHaptics | **No** | — | Use `WKInterfaceDevice.current().play(_:)` for preset haptic types instead. |

### XcodeGen watchOS Specifics (Validated)

| Aspect | Detail |
|--------|--------|
| Platform value | `platform: watchOS` (first-class enum value) |
| `supportedDestinations` | **Explicitly rejected** by XcodeGen validation for watchOS app targets. Two separate validation rules enforce this. |
| Standalone product type | `application.watchapp2-container` (Xcode 14+, watchOS 7+) |
| Paired product type | `application.watchapp2` (watch app) + iOS `application` parent |
| Legacy extension type | `watchkit2-extension` — **not needed** for modern single-target apps |
| Deployment target | `options.deploymentTarget.watchOS: "X.Y"` (project-wide) or per-target `deploymentTarget` |
| Watch embedding | Automatic when iOS target declares dependency on watchOS target. XcodeGen creates "Embed Watch Content" copy-files build phase. |
| Widget extension type | Standard `app-extension` with `com.apple.widgetkit-extension` point identifier |

---

## Locked Decisions

| Decision | Choice |
|----------|--------|
| Platform to add now | watchOS (tvOS deferred) |
| Rollout scope | Full build/edit/fix parity |
| Unsupported feature behavior | Fail early with clear reasons |
| Watch project shapes | Both: `watch_only` and `paired_ios_watch` |
| Paired project default | Standard wiring only (no auto WatchConnectivity scaffolding) |
| Schema direction | Add explicit `platform` field (not overload `device_family`) |
| Watch widgets/complications | Yes, phase 1 |
| Feature selection strictness | Conservative + explicit validation |
| Deployment target | watchOS 26+ (Apple unified all platform versions at WWDC25) |
| Build strategy for paired projects | Single unified scheme (matches Xcode template behavior) |
| Watch app target structure | Modern single-target (Xcode 14+), not legacy WatchKit App + Extension split |
| Standalone product type | `application.watchapp2-container` |
| Paired product type | `application.watchapp2` with iOS parent dependency |
| Bundle ID convention | Watch: `{ios_bundle_id}.watchkitapp` (prefix rule required by Apple) |

---

## Goal & Success Criteria

### Goal

Generate watchOS projects safely by preventing invalid feature selections before codegen and by making all Claude workflows (build/edit/fix/review/config) platform-aware.

### Success Criteria

1. Planner produces valid watchOS plans using an explicit `platform` field
2. Unsupported watchOS features fail during planning/validation with clear messages
3. XcodeGen output uses correct watchOS targets/platforms (no `supportedDestinations` misuse, correct product types)
4. Edit/fix/build prompts use watchOS-specific build commands when the project is watchOS
5. Generated project scaffold/docs/commands reflect watchOS constraints and workflows
6. Asset catalog uses watchOS-specific icon sizes and complication image sets
7. Modern single-target watch app structure used (not legacy dual-target)
8. Existing iOS behavior remains fully backward-compatible

---

## Scope

### In Scope

- Planner/analyzer/coder prompt schema and constraints
- Orchestration types + validation
- XcodeGen generation in orchestration and xcodegen MCP server
- `project_config.json` schema + xcodegen MCP tool behavior
- Build/edit/fix prompt command generation (platform-aware)
- Generated Claude scaffold (memory/docs/commands/Makefile/scripts) platform-aware text
- Feature compatibility gating for watchOS (feature keys + extension kinds)
- watchOS asset catalog variant (icon sizes, complication image sets)

### Out of Scope (Phase 1)

- tvOS
- Automatic WatchConnectivity sync scaffolding
- Full watch data mirroring templates
- Broad expansion of watchOS feature support beyond the vetted matrix

---

## Important Contract Changes

### 1. Planner Output Schema (`PlannerResult` JSON)

**Add:**
- `platform` (`"ios"` | `"watchos"`) — default `"ios"` if missing
- `watch_project_shape` (`"watch_only"` | `"paired_ios_watch"`) — only valid when `platform == "watchos"`

**Keep:**
- `device_family` for iOS only (`iphone` | `ipad` | `universal`)

**Validation rules:**

| Condition | Rule |
|-----------|------|
| `platform == "ios"` | `device_family` defaults to `"iphone"`; `watch_project_shape` must be empty |
| `platform == "watchos"` | `watch_project_shape` defaults to `"watch_only"`; `device_family` must be empty (fail if planner emits iOS values); watch feature/extension validation is mandatory |

### 2. XcodeGen MCP `project_config.json` Schema

**Add:**
- `platform` (`"ios"` | `"watchos"`)
- `watch_project_shape` (`"watch_only"` | `"paired_ios_watch"`, optional)

**Keep:** `device_family` for iOS only.

**Backward compatibility:** If `platform` is missing in existing config, default to `"ios"`.

### 3. Generated Project Memory/Docs

Platform-aware generated content in:
- CLAUDE.md memory wording
- Build command references
- Project overview / xcodegen policy / workflow docs
- Slash commands (`/build-green`, `/fix-build`, `/quality-review`, etc.)

---

## Design Overview

The watchOS implementation is split into two core tracks:

```
Platform Plumbing                    Feature Safety
├── schema/types                     ├── compatibility matrix (rule_keys)
├── XcodeGen generation              ├── extension kind validation
│   ├── watchapp2-container          ├── early failure messages
│   └── watchapp2 + iOS parent       └── prompt constraints
├── build commands
├── MCP config and tooling
└── asset catalog variant
```

This prevents "technically generated but invalid" watch projects.

---

## Implementation Plan

### 1. Platform & Watch Shape in Data Model

#### 1.1 Update Orchestration Types

**File:** `types.go`

- Add `Platform string` to `PlannerResult`
- Add `WatchProjectShape string` to `PlannerResult`
- Add helpers:
  - `GetPlatform() string` — defaults to `"ios"`
  - `GetWatchProjectShape() string` — defaults to `"watch_only"` when platform is watchOS
- Keep `GetDeviceFamily()` for iOS only; add validation to reject device family on watchOS plans

#### 1.2 Update `BuildResult`

**File:** `types.go`

- Add `Platform string`
- Add `WatchProjectShape string`
- Keep `DeviceFamily` for iOS compatibility

---

### 2. Feature Compatibility Gating (Core Safety)

#### 2.1 Platform Feature Policy Matrix

**New file:** `platform_features.go`

Define:
- Feature status per platform (`ios`, `watchos`)
- Extension kind status per platform
- Failure reason strings
- Optional notes (`"requires paired iPhone"`, `"deferred"`)

**Status enum:**

| Status | Meaning |
|--------|---------|
| `supported` | Safe to use on this platform |
| `conditional` | Supported with caveats (documented) |
| `unsupported` | Will not work; fail with reason |
| `unverified` | Treated as fail in phase 1 until researched |

#### 2.2 watchOS Phase-1 Feature Policy

> Intentionally strict to avoid invalid watch projects. Verified against Apple docs.

**Supported (watch-safe baseline):**
- `notifications` — local/on-device via UserNotifications (watchOS 3+)
- `localization`, `timers`, `healthkit`, `storage`
- `siri_intents` — via App Intents (watchOS 9+)
- `website_links`
- `maps` — SwiftUI Map view (watchOS 7+); major expansion in watchOS 26
- `charts` — Swift Charts (watchOS 9+)
- UI refinement: `accessibility`, `typography`, `color_contrast`, `spacing_layout`, `feedback_states`, `view_complexity`, `view_composition`, `gestures`, `animations`

**Conditional (supported with caveats):**
- `haptics` — supported via `WKInterfaceDevice.current().play(_:)` preset types only (not CoreHaptics custom patterns)
- `biometrics` — available as wrist detection via LocalAuthentication (watchOS 9+), not Face ID/Touch ID. Reclassified from unsupported.

**Unsupported (phase 1):**
- `camera` — no camera hardware on Apple Watch; AVFoundation on watchOS is audio-only
- `foundation_models` — iOS 26+/iPadOS 26+/macOS 26+ only
- `apple_translation` — not available on watchOS
- `adaptive_layout` — iPad/iPhone-specific
- `liquid_glass` — iOS 26 design system, not watchOS

**Unsupported (verified, not just unverified):**
- `speech` — Speech framework not available on watchOS at all
- `app_review` — `RequestReviewAction` not available on watchOS (StoreKit IAP works, review prompts do not)

**Unverified (fail early until researched):**
- `dark_mode` — custom per-app appearance toggles; phase-1 conservative block

#### 2.3 Extension Kind Compatibility (watchOS)

| Extension Kind | watchOS Status | Notes |
|---------------|---------------|-------|
| `widget` | Supported | Watch widgets/complications via WidgetKit. Families: `.accessoryCircular`, `.accessoryRectangular`, `.accessoryInline`, `.accessoryCorner` (watchOS-exclusive). Extension point: `com.apple.widgetkit-extension`. |
| `live_activity` | Unsupported | Apple Watch displays paired iPhone Live Activities via Smart Stack only (iOS 18+/watchOS 11+). No watch-native ActivityKit. Cannot start Live Activities from watchOS. |
| `share` | Unsupported | iOS-only in phase 1 |
| `notification_service` | Unsupported | iOS-only in phase 1 |
| `safari` | Unsupported | No Safari on watchOS |
| `app_clip` | Unsupported | iOS-only |
| Unknown kinds | Fail early | — |

#### 2.4 Enforcement Points

- Planner result validation (before project files are written)
- `writeProjectConfig(...)` (sanity check)
- XcodeGen MCP `add_extension` tool
- XcodeGen MCP `add_permission` / feature-related helpers when platform-sensitive
- Edit/fix flows when user requests unsupported watch feature (fail with explicit reason)

---

### 3. Prompt Changes (Platform-Aware)

#### 3.1 Planner Schema Prompt

**File:** `prompts.go`

- Add `platform` to planner JSON output contract
- Add `watch_project_shape` when `platform == "watchos"`
- Update example JSON to include `platform`
- Replace `"No watchOS/tvOS"` restriction with platform selection rules

#### 3.2 Planning Constraints

**File:** `prompts.go`

- iOS remains default
- watchOS allowed only when user explicitly requests watch/watchOS/Apple Watch
- `device_family` applies only to iOS
- `watch_project_shape` applies only to watchOS
- Enforce watchOS feature matrix + extension matrix in prompt

#### 3.3 Coder Prompt (Build/Edit/Fix)

**File:** `prompts.go`

Make prompt platform-aware:
- Avoid `"expert iOS developer"` hardcoding for watchOS runs
- Add watchOS-specific coding constraints:
  - Short interactions and glanceable flows
  - Avoid iPhone-size layout assumptions
  - Support Digital Crown / watch interaction paradigms
  - Avoid generating iOS-only UI APIs when platform is watchOS
  - Use `WKInterfaceDevice.current().play(_:)` for haptics, not CoreHaptics
  - Use wrist detection for authentication, not biometric APIs
- Keep Apple docs verification rule; add platform availability check reminder

#### 3.4 Rule Auto-Injection

**File:** `pipeline.go`

- Current iPad/universal auto-injection of `adaptive_layout` must remain iOS-only
- Guard: only inject `adaptive_layout` when `platform == "ios"` AND `device_family in ("ipad", "universal")`

---

### 4. XcodeGen Generation (Orchestration)

#### 4.1 Platform-Specific YAML Helpers

**File:** `xcodegen.go`

Refactor into platform-aware helpers:

- **iOS path** (existing): `writeIOSDestinationSettings(...)`, `deviceFamilyBuildSettings(...)`
- **watchOS path** (new):
  - No `supportedDestinations` (XcodeGen validation explicitly rejects this for watchOS)
  - `platform: watchOS`
  - `options.deploymentTarget.watchOS: "26.0"`
  - watchOS-specific build settings
  - No iOS orientation/device-family settings

#### 4.2 Watch Project Target Graphs

**File:** `xcodegen.go`

Two templates using modern single-target structure (Xcode 14+):

**`watch_only` (standalone):**
```yaml
targets:
  {AppName}:
    type: application.watchapp2-container
    platform: watchOS
    deploymentTarget: "26.0"
    sources: {AppName}
    settings:
      PRODUCT_BUNDLE_IDENTIFIER: {bundle_id}
      # WKWatchOnly = YES (set via Info.plist)
```
- Single watch app target (no legacy WatchKit Extension)
- Optional widget extension target when requested
- Shared sources group only when extensions exist

**`paired_ios_watch`:**
```yaml
targets:
  {AppName}:
    type: application
    platform: iOS
    dependencies:
      - target: {AppName}Watch
  {AppName}Watch:
    type: application.watchapp2
    platform: watchOS
    deploymentTarget: "26.0"
    sources: {AppName}Watch
    settings:
      PRODUCT_BUNDLE_IDENTIFIER: {bundle_id}.watchkitapp
      # WKRunsIndependentlyOfCompanionApp = YES
```
- iOS app target with dependency on watch target
- XcodeGen auto-creates "Embed Watch Content" build phase
- Single unified scheme for building both targets
- Bundle ID: watch must be prefixed by iOS bundle ID
- Optional widget target with strict validation
- No auto WatchConnectivity scaffolding

> **Critical (Phase 0):** Implementer must verify these target graphs against a real Xcode 16+ generated project before final coding. The YAML above is based on XcodeGen docs/fixtures but must be validated end-to-end.

#### 4.3 Watch Widget/Complication Extension Targets

Widget extension for watchOS:
```yaml
  {AppName}Widgets:
    type: app-extension
    platform: watchOS
    sources: {AppName}Widgets
    settings:
      PRODUCT_BUNDLE_IDENTIFIER: {bundle_id}.widgets
    info:
      properties:
        NSExtension:
          NSExtensionPointIdentifier: com.apple.widgetkit-extension
```
- Supports watch-specific families including `.accessoryCorner` (watchOS-exclusive)
- Fail early for non-widget extension kinds on watchOS

#### 4.4 iOS Regression Safety

Existing iPhone/iPad/universal destination filter behavior remains unchanged.

---

### 5. XcodeGen MCP Server Support

#### 5.1 `ProjectConfig` Schema Updates

**Files:** `config.go`, `setup.go` (`writeProjectConfig`)

- Add `Platform`, `WatchProjectShape`
- Default missing `platform` to `"ios"`

#### 5.2 Platform-Aware `project.yml` Generation

**File:** `config.go`

- iOS generation path (existing, unchanged)
- watchOS generation path (new, mirroring orchestration logic)
- No `supportedDestinations` for watchOS app targets
- watchOS deployment target support

#### 5.3 MCP Tool Validation Updates

**Files:** `server.go`, `tools.go`

- Tool descriptions: remove inaccurate `"iOS only"` wording
- `add_extension`: validate platform + extension kind compatibility
- `add_permission`: allow watchOS-appropriate permissions; reject unsupported ones
- `get_project_config`: print platform and watch project shape in summary
- Invalid watchOS requests: fail early with actionable error + supported alternatives

---

### 6. Platform-Aware Build Commands

#### 6.1 Canonical Build/Test Command Helpers

**Files:** `setup.go`, `pipeline.go`, `build_prompts.go`

Replace hardcoded `generic/platform=iOS Simulator`:

| Project Type | Build Command Strategy |
|-------------|----------------------|
| iOS single app | `xcodebuild ... -destination 'generic/platform=iOS Simulator'` (current) |
| watchOS watch-only | `xcodebuild ... -destination 'generic/platform=watchOS Simulator'` |
| Paired iOS + Watch | Single scheme build — Xcode builds both targets via the unified scheme |

#### 6.2 Edit/Fix Platform Detection

**Files:** `pipeline.go`, helper in `setup.go`

- Read `project_config.json` `platform` first
- Fallback to `"ios"` for legacy projects with no `platform` field

---

### 7. Generated Project Scaffold (Platform-Aware)

#### 7.1 Memory/Docs Wording

**File:** `setup.go` (memory/doc writers)

Replace iOS-only wording in:
- `project-overview.md`
- `build-fix-workflow.md`
- `quality-gates.md`
- `claude-workflow.md`
- Command descriptions (`/preflight`, `/build-green`, `/fix-build`, `/quality-review`, `/research-apple-api`)

Add watchOS-specific notes when `platform == "watchos"`:
- Feature selection constraints
- Widget/complication support path (including `.accessoryCorner` watchOS-exclusive family)
- Unsupported feature failure policy
- Paired project: "no sync scaffolding unless requested"
- Haptics: use `WKInterfaceDevice.play(_:)`, not CoreHaptics
- Auth: use wrist detection, not biometrics

#### 7.2 Review/A11y Commands for watchOS

Existing review/a11y scaffold is reusable. Generated command text should mention watchOS when applicable:
- Focus on watch interaction model (Digital Crown, small screen, glanceable)
- Compact UI constraints
- Watch-specific accessibility (touch targets, clarity on small screen)

No new watch-specific review scripts required in phase 1.

#### 7.3 Asset Catalog

**File:** `setup.go` (`writeAssetCatalog`)

Add watchOS variant:
- watchOS app icon sizes (different from iOS 1024x1024 single icon)
- Complication image sets when widget extension is present
- Separate `Contents.json` structure for watch asset catalog

#### 7.4 Makefile / CI / Scripts

**Files:** `setup.go` (`writeProjectMakefile`, `writeCIWorkflow`, `writeClaudeScripts`)

- `run-build-check.sh`: use platform-aware build command (not hardcoded iOS)
- `Makefile`: `claude-check` target uses platform-aware build helper
- CI workflow: build step uses correct simulator destination per platform
- For paired projects: build step uses unified scheme

---

### 8. Error Reporting for Invalid Feature Selection

#### 8.1 Validation Error Surface

If planner returns invalid watchOS features/rule_keys/extensions, fail before project generation and print:
- Invalid key name
- Status (`unsupported` / `unverified`)
- Reason
- Suggested alternatives or deferred path

#### 8.2 Example Error Messages

| Request | Message |
|---------|---------|
| `live_activities` on watchOS | "Unsupported for watch-native target generation. Apple Watch displays iPhone-originated Live Activities via Smart Stack only. Consider WidgetKit widget/complication instead." |
| `foundation_models` on watchOS | "Unsupported. Foundation Models framework targets iOS 26+/iPadOS 26+/macOS 26+ only." |
| `camera` on watchOS | "Unsupported. Apple Watch has no camera hardware; AVFoundation on watchOS is audio-only." |
| `speech` on watchOS | "Unsupported. Speech framework is not available on watchOS." |
| `app_review` on watchOS | "Unsupported. RequestReviewAction is not available on watchOS." |
| `haptics` on watchOS | "Supported with caveats. Use WKInterfaceDevice.current().play(_:) for preset haptic types. CoreHaptics custom patterns are not available." |

---

### 9. watchOS Planning Rules

#### 9.1 Planning Heuristics

**File:** `prompts.go`

- Favor short-session, glanceable features
- Avoid phone-centric flows as default
- Avoid generating features blocked by watch matrix
- Require explicit user intent for paired iPhone features / cross-device sync
- If paired project selected but no sync requested, do not invent WatchConnectivity code

#### 9.2 Widget/Complication Planning

When user requests "complication", "widget", "Smart Stack", etc. on watchOS:
- Planner emits watch-compatible widget extension plan (`widget`)
- Include relevant rule key (`widgets`)
- Include watchOS-exclusive `.accessoryCorner` family in widget configuration
- Reject `live_activities` unless user explicitly asks for paired iPhone Live Activities and platform includes iOS

---

## Testing & Validation

### A. Schema / Type Compatibility Tests

- Missing `platform` defaults to `"ios"`
- `platform=watchos` + `device_family=iphone` fails
- `platform=ios` + `watch_project_shape` set fails
- Legacy planner outputs still parse

### B. Feature Matrix Validation Tests

- watchOS + `camera` -> fail early with clear message
- watchOS + `foundation_models` -> fail early
- watchOS + `speech` -> fail early (verified unavailable)
- watchOS + `app_review` -> fail early (verified unavailable)
- watchOS + `widgets` -> allowed
- watchOS + `live_activities` -> fail early
- watchOS + `maps` -> allowed (verified available watchOS 2+)
- watchOS + `charts` -> allowed (verified available watchOS 9+)
- watchOS + `haptics` -> allowed with conditional status
- watchOS + `biometrics` -> allowed with conditional status (wrist detection)
- watchOS + unverified keys (`dark_mode`) -> fail early in phase 1

### C. XcodeGen YAML Generation Tests (Orchestration)

- watchOS standalone uses `type: application.watchapp2-container`
- watchOS paired uses `type: application.watchapp2` with iOS parent dependency
- watchOS app targets do not emit `supportedDestinations`
- `deploymentTarget.watchOS` is present for watch projects
- iOS projects still emit destination filters for iphone/ipad/universal (regression)
- Watch widget extension emits `app-extension` with `com.apple.widgetkit-extension`
- Paired project: XcodeGen creates "Embed Watch Content" build phase automatically
- Watch bundle ID uses `{ios_bundle_id}.watchkitapp` prefix convention

### D. XcodeGen MCP Config Tests

**New file:** `config_test.go`

- `ProjectConfig` round-trip with `platform` + `watch_project_shape`
- watchOS `generateProjectYAML(cfg)` emits correct watchOS product types
- `add_extension` rejects unsupported kinds on watchOS
- `get_project_config` summary includes platform fields

### E. Build/Edit/Fix Prompt Tests

- Platform-aware build command helper returns:
  - iOS Simulator destination for iOS projects
  - watchOS Simulator destination for watch-only projects
  - Unified scheme build for paired projects
- Edit/fix fallback to iOS for legacy `project_config.json` without `platform`

### F. Generated Scaffold Tests

Extend `setup_test.go`:

- `project-overview.md` reflects watchOS platform
- `claude-workflow.md` uses watchOS build command(s)
- `.claude/commands/*` generated text is platform-appropriate
- `Makefile` `claude-check` uses platform-aware build helper
- Asset catalog uses watchOS icon structure when platform is watchOS

### G. Manual Validation Matrix (Required Before Merge)

| Scenario | Validates |
|----------|-----------|
| iOS iPhone-only | Regression |
| iOS universal | Regression |
| watchOS watch-only app | New — uses `application.watchapp2-container` |
| watchOS watch-only + widget/complication | New — widget with `.accessoryCorner` |
| watchOS paired iPhone + Watch (no sync) | New — auto embed, unified scheme |
| watchOS paired + unsupported extension | Must fail early |

**Validation steps per scenario:**
1. `xcodegen generate`
2. Open project in Xcode — inspect targets/schemes/product types
3. Build with canonical commands (correct simulator destination)
4. Verify generated Claude scaffold commands/docs match platform
5. Confirm unsupported feature messaging is clear and early
6. Verify bundle ID prefix convention for paired projects

---

## Rollout Plan (Implementation Order)

### Phase 0: Research Lock + Target Graph Verification

> **This phase is critical and must not be skipped.**

- Create a real Xcode 16+ watch SwiftUI project (both shapes) and capture the exact target graph
- Verify that `application.watchapp2-container` is the correct product type for standalone
- Verify that `application.watchapp2` with iOS parent is correct for paired
- Confirm single-target structure (no legacy WatchKit Extension split)
- Document target types, schemes, settings, bundle ID conventions, embedding relationships
- Verify `WKWatchOnly` vs `WKRunsIndependentlyOfCompanionApp` Info.plist keys
- Finalize watch extension compatibility matrix in code
- Confirm watchOS 26 deployment target value

### Phase 1: Platform Schema + Validation

- Add `platform` and `watch_project_shape` to planner + project config types
- Add feature/extension compatibility matrix (`platform_features.go`)
- Fail early on unsupported/unverified watch selections
- Include verified status for `speech` (unsupported) and `app_review` (unsupported)

### Phase 2: XcodeGen Generation (Orchestration + MCP)

- Add watchOS `project.yml` generation paths with correct product types
- Add watch widget/complication support (including `.accessoryCorner`)
- Add watchOS asset catalog variant
- Add MCP tool platform validation and schema handling

### Phase 3: Build/Edit/Fix Parity

- Platform-aware canonical build commands (watchOS Simulator destination)
- Update pipeline edit/fix/build prompts
- Legacy config fallback to iOS

### Phase 4: Generated Scaffold Platform-Awareness

- Memory/docs/commands/Makefile/scripts platform text updates
- Watch-specific workflow notes (haptics via WKInterfaceDevice, wrist detection auth)
- Platform-aware `run-build-check.sh` and CI workflow

### Phase 5: Regression + Manual Validation

- Run full test suite
- Execute manual validation matrix
- Validate Xcode opens/builds for watch projects and iOS regressions

---

## Assumptions & Defaults

| Assumption | Value |
|-----------|-------|
| Default platform | `"ios"` (backward compatible) |
| watchOS deployment target | watchOS 26+ (Apple unified all platform versions at WWDC25) |
| `device_family` scope | iOS only |
| Paired project sync | No auto WatchConnectivity scaffolding |
| Unverified features | Fail early (not silent downgrade) |
| Paired project build | Single unified scheme (matches Xcode templates) |
| Asset catalog | watchOS-specific variant with correct icon/complication sizes |
| Watch app structure | Modern single-target (Xcode 14+), not legacy dual-target |
| Standalone product type | `application.watchapp2-container` |
| Paired product type | `application.watchapp2` + iOS `application` parent |
| Bundle ID convention | `{ios_bundle_id}.watchkitapp` (Apple prefix rule) |
| Haptics on watchOS | `WKInterfaceDevice.play(_:)` presets only, not CoreHaptics |
| Auth on watchOS | Wrist detection via LocalAuthentication, not biometrics |
