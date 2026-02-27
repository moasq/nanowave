<div align="center">

# Nanowave

**Describe your app. Nanowave writes the Swift.**

AI-powered Apple app generation from your terminal — one command from idea to running Xcode project.

[![Go](https://img.shields.io/badge/Go-1.26-00ADD8?style=flat&logo=go&logoColor=white)](https://go.dev)
[![Swift](https://img.shields.io/badge/Swift-6-FA7343?style=flat&logo=swift&logoColor=white)](https://swift.org)
[![Powered by Claude](https://img.shields.io/badge/Powered%20by-Claude%20Code-7C3AED?style=flat&logo=anthropic&logoColor=white)](https://docs.anthropic.com/en/docs/claude-code)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg?style=flat)](LICENSE)
[![macOS](https://img.shields.io/badge/macOS-only-000000?style=flat&logo=apple&logoColor=white)](https://developer.apple.com)

</div>

---

```
$ nanowave

> A workout tracker that logs exercises, tracks sets and reps,
  and shows weekly progress with charts

  ✓ Analyzed: FitTrack
  ✓ Plan ready (14 files, 5 models)
  ✓ Build complete — 14 files
  ✓ FitTrack is ready!
```

No boilerplate, no Xcode templates. Nanowave takes a sentence, plans the architecture, generates SwiftUI code, and compiles a working Xcode project — with auto-fix if the build fails. It runs on your existing [Claude Pro/Max subscription](https://claude.ai) through Claude Code.

## Platforms

Build for every Apple platform from a single prompt. Mention the target and Nanowave handles the rest.

<table>
<tr>
<td align="center"><img src="https://img.shields.io/badge/-iPhone-000?style=for-the-badge&logo=apple&logoColor=white" alt="iPhone"><br><sub>Default</sub></td>
<td align="center"><img src="https://img.shields.io/badge/-iPad-000?style=for-the-badge&logo=apple&logoColor=white" alt="iPad"><br><sub>"iPad"</sub></td>
<td align="center"><img src="https://img.shields.io/badge/-Apple%20Watch-000?style=for-the-badge&logo=apple&logoColor=white" alt="Apple Watch"><br><sub>"Apple Watch"</sub></td>
<td align="center"><img src="https://img.shields.io/badge/-Mac-000?style=for-the-badge&logo=apple&logoColor=white" alt="Mac"><br><sub>"Mac"</sub></td>
<td align="center"><img src="https://img.shields.io/badge/-Apple%20TV-000?style=for-the-badge&logo=apple&logoColor=white" alt="Apple TV"><br><sub>"Apple TV"</sub></td>
<td align="center"><img src="https://img.shields.io/badge/-Vision%20Pro-000?style=for-the-badge&logo=apple&logoColor=white" alt="Vision Pro"><br><sub>"Vision Pro"</sub></td>
</tr>
</table>

Combine any platforms in a single prompt. Each gets its own source directory, target, and Xcode scheme.

## What it generates

Every project ships with:

- **SwiftUI** views with `#Preview` blocks, navigation, and state management
- **SwiftData** models (when persistence is needed)
- **Supabase** backend — auth (Apple Sign In, email, anonymous), database, storage, and realtime via MCP
- **Swift Charts** for data visualization
- **AppTheme** design system — centralized colors, fonts, spacing tokens
- **SF Symbols** — appropriate icons for every screen
- **XcodeGen** project spec — reproducible `.xcodeproj` generation
- **Asset catalog** with AppIcon and AccentColor
- **SPM packages** from a curated, validated registry (when native frameworks aren't enough)

Apps use Apple-first frameworks wherever possible:

<table>
<tr>
<td align="center"><a href="https://developer.apple.com/xcode/swiftui/"><img src="https://developer.apple.com/assets/elements/icons/swiftui/swiftui-96x96_2x.png" width="40"><br><b>SwiftUI</b></a><br><sub>UI</sub></td>
<td align="center"><a href="https://developer.apple.com/xcode/swiftdata/"><img src="https://developer.apple.com/assets/elements/icons/swiftdata/swiftdata-96x96_2x.png" width="40"><br><b>SwiftData</b></a><br><sub>Persistence</sub></td>
<td align="center"><a href="https://developer.apple.com/documentation/charts"><img src="https://developer.apple.com/assets/elements/icons/swift-charts/swift-charts-96x96_2x.png" width="40"><br><b>Swift Charts</b></a><br><sub>Data viz</sub></td>
<td align="center"><a href="https://developer.apple.com/documentation/mapkit"><img src="https://developer.apple.com/assets/elements/icons/mapkit/mapkit-96x96_2x.png" width="40"><br><b>MapKit</b></a><br><sub>Maps</sub></td>
<td align="center"><a href="https://developer.apple.com/documentation/healthkit"><img src="https://developer.apple.com/assets/elements/icons/healthkit/healthkit-96x96_2x.png" width="40"><br><b>HealthKit</b></a><br><sub>Health</sub></td>
<td align="center"><a href="https://developer.apple.com/documentation/widgetkit"><img src="https://developer.apple.com/assets/elements/icons/widgetkit/widgetkit-96x96_2x.png" width="40"><br><b>WidgetKit</b></a><br><sub>Widgets</sub></td>
</tr>
<tr>
<td align="center"><a href="https://developer.apple.com/documentation/avfoundation"><img src="https://developer.apple.com/assets/elements/icons/avfoundation/avfoundation-96x96_2x.png" width="40"><br><b>AVFoundation</b></a><br><sub>Camera & media</sub></td>
<td align="center"><a href="https://developer.apple.com/documentation/photokit"><img src="https://developer.apple.com/assets/elements/icons/photokit/photokit-96x96_2x.png" width="40"><br><b>PhotosUI</b></a><br><sub>Photo picker</sub></td>
<td align="center"><a href="https://developer.apple.com/documentation/activitykit"><img src="https://developer.apple.com/assets/elements/icons/activitykit/activitykit-96x96_2x.png" width="40"><br><b>ActivityKit</b></a><br><sub>Live Activities</sub></td>
<td align="center"><a href="https://developer.apple.com/machine-learning/core-ml/"><img src="https://developer.apple.com/assets/elements/icons/core-ml/core-ml-96x96_2x.png" width="40"><br><b>CoreML</b></a><br><sub>Machine learning</sub></td>
<td align="center"><a href="https://developer.apple.com/augmented-reality/arkit/"><img src="https://developer.apple.com/assets/elements/icons/arkit/arkit-96x96_2x.png" width="40"><br><b>ARKit</b></a><br><sub>Augmented reality</sub></td>
<td align="center"><a href="https://developer.apple.com/augmented-reality/realitykit/"><img src="https://developer.apple.com/assets/elements/icons/realitykit/realitykit-96x96_2x.png" width="40"><br><b>RealityKit</b></a><br><sub>3D & spatial</sub></td>
</tr>
<tr>
<td align="center"><a href="https://supabase.com"><img src="https://cdn.simpleicons.org/supabase/3FCF8E" width="40"><br><b>Supabase</b></a><br><sub>Backend & auth</sub></td>
</tr>
</table>

## Getting started

Install [Xcode](https://apps.apple.com/app/xcode/id497799835) if you don't have it, then:

```bash
brew install moasq/tap/nanowave
```

That's it. On first launch, Nanowave detects missing dependencies and installs them for you:

```
$ nanowave

  Nanowave Setup

  Checking Xcode...                    ✓ installed
  Checking iOS Simulator...            ✓ available
  Checking Homebrew...                 ✓ installed
  Checking Node.js...                  ✓ installed (v22.x)
  Checking Claude Code CLI...          ✗ not found
    Install Claude Code CLI via npm? [Y/n] y
    Installing Claude Code CLI...      ✓ installed
  Checking XcodeGen...                 ✗ not found
    Install XcodeGen via Homebrew? [Y/n] y
    Installing XcodeGen...             ✓ installed
  Configuring MCP servers...
  Setting up XcodeBuildMCP...          ✓ configured
  Setting up Apple Docs MCP...         ✓ configured

  ✓ All prerequisites installed! You're ready to build.

> describe your app here...
```

Run `nanowave setup` at any time to re-check or repair your environment.

<details>
<summary>Install from source</summary>

```bash
git clone https://github.com/moasq/nanowave.git
cd nanowave
make build
./bin/nanowave    # auto-runs setup on first launch
```

</details>

## Usage

### Build a new app

```bash
nanowave
```

Describe your app and watch it come to life:

```
> A habit tracker with a weekly grid, streak counter, and haptic feedback

  ✓ Analyzed: HabitGrid
  ✓ Plan ready (12 files, 3 models)
  ✓ Build complete — 12 files
  ✓ HabitGrid is ready!
  A minimal habit tracker with weekly grid view and streak tracking

  • Habit List — Browse and manage your daily habits
  • Weekly Grid — Visual grid showing completion across the week
  • Streak Counter — Track consecutive days of habit completion
  • Haptic Feedback — Satisfying tactile response on completion

  Files    12
  Location ~/nanowave/projects/HabitGrid
  ✓ Launched on iPhone 17 Pro
```

On subsequent runs, a project picker lets you resume an existing project or start a new one. All projects live in `~/nanowave/projects/`.

### Multi-platform builds

Mention multiple platforms and Nanowave generates a single Xcode project with separate targets:

```
> A weather app for iPhone, Apple Watch, Mac, Vision Pro, and Apple TV
  that shows current conditions and a 5-day forecast

  ✓ Analyzed: Skies
  ✓ Plan ready (29 files, 4 models) — iOS, watchOS, macOS, visionOS, tvOS
  ✓ Build complete — 29 files
  ✓ Skies is ready!
```

### Backend integration

Mention authentication or a database and Nanowave connects [Supabase](https://supabase.com) automatically — auth providers, tables, RLS policies, and storage buckets are provisioned before code generation begins:

```
> A mood journal with Sign in with Apple, anonymous auth, and cloud sync

  ✓ Analyzed: Aura
  ✓ Supabase connected (project: aura-xyz)
  ✓ Tables created: moods
  ✓ Auth providers: apple, anonymous
  ✓ Build complete — 10 files
  ✓ Aura is ready!
```

Manage connections with `nanowave integrations`:

```bash
nanowave integrations           # list configured integrations
nanowave integrations setup     # connect a new provider
nanowave integrations remove    # disconnect a provider
```

### Edit an existing app

Select a project from the picker, then describe changes:

```
> Add a dark mode toggle to the settings screen

  ✓ Changes applied!

  Added an appearance setting to SettingsView with system/light/dark
  options, wired to @AppStorage and applied via preferredColorScheme
  on the root view.
```

### Ask about your project

Use `/ask` to query your project without triggering an edit (lightweight, read-only, cheap):

```
> /ask how many views do I have?

  You have 8 SwiftUI views across 5 feature modules...
  $0.0012
```

### Commands

| Command | Description |
|---|---|
| `/run` | Build and launch in simulator |
| `/fix` | Auto-fix compilation errors |
| `/ask <question>` | Ask about your project (cheap, read-only) |
| `/open` | Open project in Xcode |
| `/projects` | Switch to another project |
| `/model [name]` | Switch AI model |
| `/simulator [name]` | Pick simulator device |
| `/info` | Show project info |
| `/usage` | Token usage and cost |
| `/clear` | Clear conversation session |
| `/setup` | Install prerequisites |
| `/help` | Show all commands |

```bash
# CLI subcommands
nanowave              # interactive mode (default)
nanowave chat         # same as above
nanowave fix          # auto-fix build errors for most recent project
nanowave run          # build and launch most recent project in simulator
nanowave info         # show project status
nanowave open         # open most recent project in Xcode
nanowave usage        # show usage and cost history
nanowave integrations # manage backend integrations (Supabase, etc.)
nanowave setup        # install and verify prerequisites

# Flags
nanowave --model <name>  # use a specific AI model
nanowave --version       # print version
nanowave --help          # print help
```

## How it works

Nanowave runs a multi-phase AI pipeline — from natural language to compiled Xcode project:

```
describe  →  analyze  →  plan  →  build  →  fix  →  run
   ↑            │          │        │        │       │
 prompt     app name    files    Swift    xcode-   iOS
            features    models   code     build    Simulator
            core flow   palette  assets   errors
                        nav      .xcproj  auto-fix
```

| Phase | What happens |
|-------|-------------|
| **Analyze** | Extracts app name, features, core flow, and deferred items from your description |
| **Plan** | Produces a file-level build plan: data models, file layout, color palette, navigation, packages |
| **Build** | Generates Swift source files, `project.yml`, asset catalog, and runs XcodeGen (up to 6 completion passes) |
| **Fix** | Compiles with `xcodebuild`, reads errors, and auto-repairs until the build is green |
| **Run** | Boots the iOS Simulator, installs, and launches the app |

Edits follow the same pipeline: the AI reads the existing project, applies changes, rebuilds, and auto-fixes. The `/ask` command uses a lightweight model with read-only tools for cheap project exploration.

## Project structure

Generated projects follow a consistent layout:

```
~/nanowave/projects/
└── HabitGrid/
    ├── .nanowave/           # project state, history, usage
    ├── .claude/             # CLAUDE.md, skills, MCP config
    ├── HabitGrid/
    │   ├── App/             # @main entry point, root views
    │   ├── Features/        # screens grouped by feature
    │   ├── Models/          # SwiftData / in-memory models
    │   └── Theme/           # AppTheme (colors, fonts, spacing)
    ├── project.yml          # XcodeGen spec
    ├── .gitignore
    └── HabitGrid.xcodeproj
```

Multi-platform projects add per-platform source directories + a `Shared/` folder:

```
└── Skies/
    ├── Skies/               # iOS source
    ├── SkiesWatch/          # watchOS source
    ├── SkiesMac/            # macOS source
    ├── SkiesVision/         # visionOS source
    ├── SkiesTV/             # tvOS source
    ├── Shared/              # cross-platform code
    ├── project.yml          # multi-target XcodeGen spec
    └── Skies.xcodeproj
```

## Cost

Nanowave runs on your existing [Claude Pro or Max subscription](https://claude.ai) through [Claude Code](https://docs.anthropic.com/en/docs/claude-code). No additional API charges. Use `/usage` to track token consumption per session.

## Development

```bash
git clone https://github.com/moasq/nanowave.git
cd nanowave

make build          # build binary to ./bin/nanowave
make test           # run tests
make run ARGS="--help"  # run from source
make deps           # tidy go modules
make clean          # remove build artifacts
```

<details>
<summary>Project layout</summary>

```
cmd/nanowave/           # CLI entry point (cobra)
internal/
├── claude/             # Claude Code client (streaming, sessions)
├── commands/           # Cobra commands (root, chat, fix, run, setup, etc.)
├── config/             # Environment detection, project catalog
├── integrations/       # Backend integrations (Supabase) + secret store
├── orchestration/      # Multi-phase build pipeline
│   └── skills/         # Embedded AI skill files (70+ skills)
├── service/            # Service layer (build, edit, fix, run, info)
├── storage/            # JSON-file stores (project, history, usage)
├── terminal/           # UI (spinner, picker, input, colors)
└── xcodegenserver/     # XcodeGen MCP server
```

</details>

## License

MIT
