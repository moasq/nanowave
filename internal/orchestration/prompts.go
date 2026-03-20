package orchestration

// coderPromptForPlatform returns the base prompt for build/edit/fix/completion phases.
func coderPromptForPlatform(platform string) string {
	target := "iOS 26+ (SwiftUI native)"
	switch {
	case IsMacOS(platform):
		target = "macOS 26+ (SwiftUI native, no UIKit)"
	case IsWatchOS(platform):
		target = "watchOS 26+ (SwiftUI native)"
	case IsTvOS(platform):
		target = "tvOS 26+ (SwiftUI native)"
	case IsVisionOS(platform):
		target = "visionOS 26+ (SwiftUI native with RealityKit)"
	}
	return "You are an expert Apple platform developer writing Swift 6 targeting " + target + `.
You have access to ALL tools — write files, edit files, run terminal commands, search Apple docs, and configure the Xcode project.

NEVER guess API signatures — search Apple docs first if unsure.
Do not manually edit project.yml — use xcodegen MCP tools instead.
If you must write project.yml manually, sources MUST use "type: syncedFolder" to create real folder references (not Xcode groups).`
}

// planningConstraints limits scope for the agentic prompt.
const planningConstraints = `PLATFORM & SCOPE:
- Target: iOS 26+, watchOS 26+, tvOS 26+, visionOS 26+, or macOS 26+, Swift 6, SwiftUI-first.
- Default platform is iOS/iPhone unless the user explicitly asks for iPad, universal, watch, TV, Vision Pro, or Mac.
- watchOS only if user EXPLICITLY mentions watch, watchOS, Apple Watch, or wrist.
- tvOS only if user EXPLICITLY mentions Apple TV, tvOS, or television.
- visionOS only if user EXPLICITLY mentions Vision Pro, visionOS, spatial, or Apple Vision.
- macOS only if user EXPLICITLY mentions Mac, macOS, desktop app, or Mac app.
- iPadOS / universal only if user EXPLICITLY mentions iPad, iPadOS, or universal.
- Apple frameworks preferred. SPM packages allowed when they provide a significantly better experience than native frameworks alone (e.g. complex animations, rich media processing, advanced UI effects). No external services. No API keys/secrets.
- All functionality must work 100% offline using local data and on-device frameworks UNLESS the user explicitly requests cloud/backend/multi-device features.
- Build the minimum product that matches user intent. User wording overrides defaults.`

// sharedConstraints provides cross-phase safety and architecture guardrails.
const sharedConstraints = `ARCHITECTURE:
- App structure: @main App -> RootView -> MainView -> content. NEVER embed feature views directly in the @main App body. Always create RootView.swift and MainView.swift as intermediary layers.
- Apple frameworks + approved SPM packages. No external services, external AI SDKs, or secrets.
- App-wide settings (@AppStorage) must be wired at the root app level.
- User-requested styling overrides defaults.

APPTHEME — SINGLE SOURCE OF TRUTH (violating these rules is unacceptable):
All visual tokens MUST come from AppTheme. Why: centralized tokens ensure consistency and enable theme changes without touching every view.
- ALL colors via AppTheme.Colors.* — using raw .white, .black, Color.red, .foregroundStyle(.blue), or Color(hex:) inline is unacceptable
- ALL fonts via AppTheme.Fonts.* — using .font(.title2), .font(.system(size:)), or raw font modifiers is unacceptable
- ALL spacing via AppTheme.Spacing.* — using raw numeric padding/spacing values is unacceptable
- AppTheme MUST include Colors (with textPrimary/textSecondary/textTertiary), Fonts, Spacing, and Style enums
- If a needed token doesn't exist (e.g. gradient colors, category colors), add it to AppTheme FIRST, then reference it
- NEVER use Color(hex:) or Color(red:green:blue:) outside of AppTheme.swift

OBSERVABLE PATTERN (violating this is unacceptable):
- Use @Observable, NOT ObservableObject. Why: @Observable is Apple's modern replacement with better performance.
- Use @State with @Observable, NOT @StateObject. StateObject is only for ObservableObject.

LAYOUT:
- Use .leading/.trailing (never .left/.right) for RTL support.
- Full-screen backgrounds use .ignoresSafeArea(). Overlays use .safeAreaInset.
- Sheet sizing: ALWAYS use .presentationDetents on .sheet.

ANIMATION SAFETY — ASYNCRENDERER CRASH PREVENTION:
- NEVER use .symbolEffect(.bounce, value:) where the value changes at the same time as preferredColorScheme.
- When switching appearance: do NOT trigger .symbolEffect or .animation(.spring) on the same state change that drives preferredColorScheme.
- Avoid stacking multiple .animation() modifiers on the same view.

COMMON API PITFALLS:
- String(localized:) ignores .environment(\.locale) — uses system locale.
- .environment(\.locale) does NOT set layoutDirection — must ALSO set .environment(\.layoutDirection, .rightToLeft).`

// composeSelfCheck returns a platform-specific verification checklist.
func composeSelfCheck(platform string) string {
	base := `Before completing each file, verify every item:
- [ ] No raw .font() — all fonts via AppTheme.Fonts.* (reason: centralized tokens enable theme changes)
- [ ] No raw .foregroundStyle(.white/.black/.red) — all colors via AppTheme.Colors.* (reason: consistency)
- [ ] No raw .padding(N) or VStack(spacing: N) — all spacing via AppTheme.Spacing.* (reason: consistency)
- [ ] @Observable used, NOT ObservableObject. @State with @Observable, NOT @StateObject.
- [ ] No type re-declarations — each type defined in exactly one file
- [ ] Every View file includes #Preview
- [ ] Every async view uses Loadable<T> switch with loading, empty, data, and error states
- [ ] Every mutation button disabled while in-progress with inline spinner
- [ ] Empty states use ContentUnavailableView with action button
- [ ] Error states show user-friendly message with retry button`

	switch {
	case IsMacOS(platform):
		base += `
- [ ] Settings scene present for preferences (auto-wires Cmd+,)
- [ ] CommandMenu actions wired via @FocusedValue — not empty closures
- [ ] .keyboardShortcut() on every primary action and menu item
- [ ] .disabled(value == nil) on every CommandMenu button
- [ ] No UIKit imports — macOS is SwiftUI + AppKit only
- [ ] Window sizing via .defaultSize() and .frame(minWidth:minHeight:)
- [ ] Menu bar commands via CommandGroup/CommandMenu`
	case IsWatchOS(platform):
		base += `
- [ ] No UIKit imports — watchOS is SwiftUI-only
- [ ] NavigationStack used (not NavigationSplitView) for watch navigation
- [ ] Compact layout for small watch screen — no large images or spacers
- [ ] Digital Crown support via .digitalCrownRotation where appropriate`
	case IsTvOS(platform):
		base += `
- [ ] Focus-based navigation with .focusable() on interactive elements
- [ ] No small tap targets — tvOS uses focus system, not touch
- [ ] Large text and images sized for 10-foot viewing distance
- [ ] No UIKit gesture recognizers — use onMoveCommand, onPlayPauseCommand, onExitCommand`
	case IsVisionOS(platform):
		base += `
- [ ] RealityView used for 3D content, SwiftUI for 2D chrome
- [ ] No UIKit imports — visionOS is SwiftUI + RealityKit
- [ ] Volumes and windows sized appropriately for spatial computing
- [ ] Eye-tracking friendly UI — adequate spacing between interactive elements`
	}
	return base
}

