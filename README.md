<div align="center">

<br>

# Nanowave

**Describe your app. Nanowave writes the Swift.**

From idea to App Store — entirely from your terminal.

<br>

[![Go](https://img.shields.io/badge/Go-1.26-00ADD8?style=flat&logo=go&logoColor=white)](https://go.dev)
[![Swift](https://img.shields.io/badge/Swift-6-FA7343?style=flat&logo=swift&logoColor=white)](https://swift.org)
[![Powered by Claude](https://img.shields.io/badge/Powered%20by-Claude%20Code-7C3AED?style=flat&logo=anthropic&logoColor=white)](https://docs.anthropic.com/en/docs/claude-code)
[![macOS](https://img.shields.io/badge/macOS-only-000000?style=flat&logo=apple&logoColor=white)](https://developer.apple.com)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg?style=flat)](LICENSE)

</div>

<br>

```
$ nanowave

> A workout tracker that logs exercises, tracks sets and reps,
  and shows weekly progress with charts

  ✓ Analyzed: FitTrack
  ✓ Plan ready (14 files, 5 models)
  ✓ Build complete — 14 files
  ✓ FitTrack is ready!
```

One sentence in, a compiled Xcode project out. Nanowave plans the architecture, generates SwiftUI code, and auto-fixes until it builds. Runs on your [Claude Pro/Max subscription](https://claude.ai) through Claude Code.

<br>

## Install

```bash
brew install moasq/tap/nanowave
```

On first launch, Nanowave detects and installs any missing dependencies automatically.

<details>
<summary>Install from source</summary>

```bash
git clone https://github.com/moasq/nanowave.git && cd nanowave
make build
./bin/nanowave
```

</details>

## Usage

Launch `nanowave`, describe your app, and it builds:

```
> A habit tracker with a weekly grid, streak counter, and haptic feedback

  ✓ HabitGrid is ready!

  Files    12
  Location ~/nanowave/projects/HabitGrid
  ✓ Launched on iPhone 17 Pro
```

Select an existing project to edit it:

```
> Add a dark mode toggle to the settings screen

  ✓ Changes applied!
```

## Platforms

Mention any Apple platform in your prompt — combine as many as you want:

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

## Deploy

Ship to the App Store and TestFlight without leaving the terminal.

<table>
<tr>
<td align="center"><img src="https://cdn.simpleicons.org/appstore/0D96F6" width="40" alt="App Store"><br><b>App Store</b><br><sub>Full submission</sub></td>
<td align="center"><img src="https://developer.apple.com/assets/elements/icons/testflight/testflight-96x96_2x.png" width="40" alt="TestFlight"><br><b>TestFlight</b><br><sub>Beta distribution</sub></td>
</tr>
</table>

```
> /connect publish to the App Store

  Nanowave Connect
  ✓ Authenticated with App Store Connect
  ✓ paytestersnow (com.example.app)
  ✓ Version 1.0 ready for submission
  ✓ Build 3 processed and ready
  ✓ App icon ready (19 sizes)
  ✓ Screenshots ready (iPhone 6.9", iPad 13")

  Submitted for App Review!
```

Nanowave handles code signing, metadata, screenshots (automatic simulator capture or browser upload), privacy declarations, and submission — with confirmation before any destructive action.

## Integrations

Mention authentication, a database, or a paid feature — Nanowave connects the backend automatically.

<table>
<tr>
<td align="center"><a href="https://supabase.com"><img src="https://cdn.simpleicons.org/supabase/3FCF8E" width="40"><br><b>Supabase</b></a><br><sub>Auth, database, storage</sub></td>
<td align="center"><a href="https://www.revenuecat.com"><img src="https://cdn.simpleicons.org/revenuecat/F25A5A" width="40"><br><b>RevenueCat</b></a><br><sub>Subscriptions & paywalls</sub></td>
</tr>
</table>

## Frameworks

Apps use Apple-first frameworks wherever possible:

<table>
<tr>
<td align="center"><a href="https://developer.apple.com/xcode/swiftui/"><img src="https://developer.apple.com/assets/elements/icons/swiftui/swiftui-96x96_2x.png" width="36"><br><b>SwiftUI</b></a><br><sub>UI</sub></td>
<td align="center"><a href="https://developer.apple.com/xcode/swiftdata/"><img src="https://developer.apple.com/assets/elements/icons/swiftdata/swiftdata-96x96_2x.png" width="36"><br><b>SwiftData</b></a><br><sub>Persistence</sub></td>
<td align="center"><a href="https://developer.apple.com/documentation/charts"><img src="https://developer.apple.com/assets/elements/icons/swift-charts/swift-charts-96x96_2x.png" width="36"><br><b>Swift Charts</b></a><br><sub>Data viz</sub></td>
<td align="center"><a href="https://developer.apple.com/documentation/mapkit"><img src="https://developer.apple.com/assets/elements/icons/mapkit/mapkit-96x96_2x.png" width="36"><br><b>MapKit</b></a><br><sub>Maps</sub></td>
<td align="center"><a href="https://developer.apple.com/documentation/healthkit"><img src="https://developer.apple.com/assets/elements/icons/healthkit/healthkit-96x96_2x.png" width="36"><br><b>HealthKit</b></a><br><sub>Health</sub></td>
<td align="center"><a href="https://developer.apple.com/documentation/widgetkit"><img src="https://developer.apple.com/assets/elements/icons/widgetkit/widgetkit-96x96_2x.png" width="36"><br><b>WidgetKit</b></a><br><sub>Widgets</sub></td>
</tr>
<tr>
<td align="center"><a href="https://developer.apple.com/documentation/avfoundation"><img src="https://developer.apple.com/assets/elements/icons/avfoundation/avfoundation-96x96_2x.png" width="36"><br><b>AVFoundation</b></a><br><sub>Camera & media</sub></td>
<td align="center"><a href="https://developer.apple.com/documentation/photokit"><img src="https://developer.apple.com/assets/elements/icons/photokit/photokit-96x96_2x.png" width="36"><br><b>PhotosUI</b></a><br><sub>Photo picker</sub></td>
<td align="center"><a href="https://developer.apple.com/documentation/activitykit"><img src="https://developer.apple.com/assets/elements/icons/activitykit/activitykit-96x96_2x.png" width="36"><br><b>ActivityKit</b></a><br><sub>Live Activities</sub></td>
<td align="center"><a href="https://developer.apple.com/machine-learning/core-ml/"><img src="https://developer.apple.com/assets/elements/icons/core-ml/core-ml-96x96_2x.png" width="36"><br><b>CoreML</b></a><br><sub>ML</sub></td>
<td align="center"><a href="https://developer.apple.com/augmented-reality/arkit/"><img src="https://developer.apple.com/assets/elements/icons/arkit/arkit-96x96_2x.png" width="36"><br><b>ARKit</b></a><br><sub>AR</sub></td>
<td align="center"><a href="https://developer.apple.com/augmented-reality/realitykit/"><img src="https://developer.apple.com/assets/elements/icons/realitykit/realitykit-96x96_2x.png" width="36"><br><b>RealityKit</b></a><br><sub>3D & spatial</sub></td>
</tr>
</table>

## How it works

```
describe  →  analyze  →  plan  →  build  →  fix  →  run
   ↑            │          │        │        │       │
 prompt     app name    files    Swift    xcode-   iOS
            features    models   code     build    Simulator
```

| Phase | What happens |
|-------|-------------|
| **Analyze** | Extracts app name, features, and core flow from your description |
| **Plan** | Produces a file-level build plan with models, navigation, and packages |
| **Build** | Generates Swift source, asset catalog, and Xcode project |
| **Fix** | Compiles with `xcodebuild` and auto-repairs until green |
| **Run** | Boots the simulator and launches the app |

## Commands

| Command | |
|---|---|
| `/run` | Build and launch in simulator |
| `/fix` | Auto-fix compilation errors |
| `/ask <question>` | Query your project (read-only) |
| `/open` | Open in Xcode |
| `/connect` | App Store Connect & TestFlight |
| `/help` | All commands |

<details>
<summary>CLI subcommands</summary>

```bash
nanowave              # interactive mode (default)
nanowave fix          # auto-fix build errors
nanowave run          # build and launch in simulator
nanowave info         # project status
nanowave open         # open in Xcode
nanowave usage        # token usage and cost
nanowave integrations # manage integrations
nanowave setup        # install prerequisites
nanowave --version    # print version
```

</details>

## Cost

Runs on your existing [Claude Pro or Max](https://claude.ai) subscription through [Claude Code](https://docs.anthropic.com/en/docs/claude-code). No additional API charges.

## Development

```bash
make build   # build binary
make test    # run tests
```

<details>
<summary>Project layout</summary>

```
cmd/nanowave/           # CLI entry point (cobra)
internal/
├── claude/             # Claude Code client
├── commands/           # Cobra commands
├── config/             # Environment detection
├── integrations/       # Supabase, RevenueCat
├── orchestration/      # Multi-phase build pipeline
│   └── skills/         # Embedded AI skills (100+)
├── service/            # Build, edit, fix, run
├── storage/            # Project state persistence
├── terminal/           # UI components
└── xcodegenserver/     # XcodeGen MCP server
```

</details>

## License

MIT
