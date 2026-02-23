# Nanowave

Autonomous app builder powered by [Claude Code](https://docs.anthropic.com/en/docs/claude-code). Describe what you want, Nanowave builds it — for iPhone, iPad, Apple Watch, and Apple TV.

```
$ nanowave

> A workout tracker that logs exercises, tracks sets and reps,
  and shows weekly progress with charts

  ✓ Analyzed: FitTrack
  ✓ Plan ready (14 files, 5 models)
  ✓ Build complete — 14 files
  ✓ FitTrack is ready!
```

One command, no boilerplate. Nanowave takes a sentence, plans the architecture, generates SwiftUI code, and compiles a working Xcode project. It uses your existing Claude subscription — no extra API costs.

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

You can also run `nanowave setup` at any time to re-check or repair your environment.

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

### Interactive mode

```bash
nanowave
```

On first run with no projects, you'll be prompted to describe your app. On subsequent runs, a project picker lets you resume an existing project or start a new one.

All projects are stored in `~/nanowave/projects/`.

### Build a new app

By default, Nanowave creates **iOS (iPhone)** apps. To target other platforms, mention them in your description.

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

#### Multi-platform builds

Mention the platforms you want and Nanowave generates a single Xcode project with separate targets:

```
> A weather app for iPhone, iPad, Apple Watch, and Apple TV that shows
  current conditions and a 5-day forecast

  ✓ Analyzed: Skies
  ✓ Plan ready (29 files, 4 models) — iOS, watchOS, tvOS
  ✓ Build complete — 29 files
  ✓ Skies is ready!
```

| Platform | How to request it |
|---|---|
| iPhone (default) | No extra description needed |
| iPad | Mention "iPad" — creates a universal iOS app |
| Apple Watch | Mention "Apple Watch" or "watchOS" |
| Apple TV | Mention "Apple TV" or "tvOS" |

You can combine any platforms in a single prompt. Each gets its own source directory, target, and scheme.

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

Use `/ask` to ask questions without triggering a full edit (uses Haiku, read-only tools, much cheaper):

```
> /ask how many views do I have?

  You have 8 SwiftUI views across 5 feature modules...
  $0.0012
```

### Slash commands

| Command | Description |
|---|---|
| `/run` | Build and launch in simulator |
| `/fix` | Auto-fix compilation errors |
| `/ask <question>` | Ask about your project (cheap, read-only) |
| `/open` | Open project in Xcode |
| `/projects` | Switch to another project |
| `/model [name]` | Switch model (`sonnet`, `opus`, `haiku`) |
| `/simulator [name]` | Pick simulator device |
| `/info` | Show project info |
| `/usage` | Token usage and cost |
| `/clear` | Clear conversation session |
| `/setup` | Install prerequisites |
| `/help` | Show all commands |
| `/quit` | Exit session |

### CLI subcommands

```bash
nanowave              # interactive mode (default)
nanowave chat         # same as above
nanowave fix          # auto-fix build errors for most recent project
nanowave run          # build and launch most recent project in simulator
nanowave info         # show project status
nanowave open         # open most recent project in Xcode
nanowave usage        # show usage and cost history
nanowave setup        # install and verify prerequisites
```

### Flags

```bash
nanowave --model opus    # use a specific Claude model for the session
nanowave --version       # print version
nanowave --help          # print help
```

## How it works

Nanowave runs a multi-phase pipeline:

```
describe → analyze → plan → build → fix → run
```

1. **Analyze** — Extracts the app name, features, core flow, and deferred items from your description (Claude Sonnet).
2. **Plan** — Produces a file-level build plan: data models, file layout, color palette, navigation structure (Claude Sonnet).
3. **Build** — Generates Swift source files, `project.yml`, asset catalog, and runs XcodeGen to produce the `.xcodeproj` (Claude Sonnet, agentic mode with up to 6 completion passes).
4. **Fix** — Compiles with `xcodebuild`, reads errors, and auto-repairs until the build succeeds.
5. **Run** — Boots the iOS Simulator, installs the app, and launches it.

Edits follow the same pattern: Claude reads the existing project, applies changes, rebuilds, and auto-fixes. After each edit, a summary of what was changed is displayed.

The `/ask` command provides a lightweight Q&A path using Claude Haiku with read-only tools (Read, Glob, Grep) — useful for exploring your project without incurring full edit costs.

## Project structure

Generated projects follow a consistent layout:

```
~/nanowave/projects/
└── HabitGrid/
    ├── .nanowave/           # project state (project.json, history, usage)
    ├── .claude/             # CLAUDE.md, skills, MCP config
    ├── HabitGrid/
    │   ├── App/             # Entry point, root views
    │   ├── Features/        # Screens grouped by feature
    │   ├── Models/          # Data models
    │   └── Theme/           # Colors, fonts, design tokens
    ├── project.yml          # XcodeGen spec
    ├── .gitignore
    └── HabitGrid.xcodeproj
```

Multi-platform projects add separate source directories per platform:

```
└── Skies/
    ├── Skies/               # iOS source
    ├── SkiesWatch/          # watchOS source
    ├── SkiesTV/             # tvOS source
    ├── Shared/              # Cross-platform code
    ├── project.yml          # Multi-target XcodeGen spec
    └── Skies.xcodeproj
```

All projects target **Swift 6**, **SwiftUI**, with zero external dependencies. Deployment targets: **iOS 26+**, **watchOS 26+**, **tvOS 26+**.

## Examples

| Example | Platforms | Description |
|---|---|---|
| [Skies](examples/skies/) | iOS, watchOS, tvOS | Multi-platform weather app with condition-driven gradients, animated icons, and ambient TV dashboard |

## Models

Nanowave uses Claude Code as its AI backend. You can switch models at any time with `/model`:

| Model | Best for |
|---|---|
| `sonnet` | Default. Fast, great for most builds and edits. |
| `opus` | Most capable. Complex architectures, nuanced UI. |
| `haiku` | Fastest. Quick edits, lightweight tasks. |

## Cost

Nanowave uses your existing [Claude Pro/Max subscription](https://claude.ai) through Claude Code. There are no additional API charges. Use `/usage` or `nanowave usage` to track token consumption and costs per session.

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

### Project layout

```
cmd/nanowave/           # CLI entry point
internal/
├── claude/             # Claude Code client (streaming, sessions)
├── commands/           # Cobra commands (root, chat, fix, run, setup, etc.)
├── config/             # Environment detection, project catalog
├── orchestration/      # Multi-phase build pipeline, prompts, scaffolding
├── service/            # Service layer (build, edit, fix, run, info)
├── storage/            # JSON-file stores (project, history, usage)
├── terminal/           # UI (spinner, picker, input, colors)
└── xcodegenserver/     # XcodeGen MCP server
```

## License

MIT
