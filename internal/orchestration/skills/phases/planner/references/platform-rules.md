# Platform Rules

## Exact Values (use these strings verbatim)
- `platform`: exactly `"ios"`, `"watchos"`, `"tvos"`, `"visionos"`, or `"macos"` — no other values
- `device_family`: exactly `"iphone"`, `"ipad"`, or `"universal"` — no other values
- `watch_project_shape`: exactly `"watch_only"` or `"paired_ios_watch"` — no other values

## iOS (default)
- Default platform is `"ios"` and device_family is `"iphone"`.

## watchOS
- Use `"watchos"` only when the user explicitly mentions Apple Watch, watchOS, watch, or wrist.
- For watchos, watch_project_shape defaults to `"watch_only"` unless the user explicitly wants a companion iPhone app.
- For a companion iPhone + Watch app, set `platform` to `"watchos"` and `watch_project_shape` to `"paired_ios_watch"`.
- Do not set device_family for watchos.
- Use tvos only when the user explicitly mentions Apple TV, tvOS, or television.
- Do not set device_family or watch_project_shape for tvos.
- tvOS apps cannot use camera, biometrics, healthkit, haptics, maps, speech, or apple-translation.
- tvOS only supports tv-top-shelf extension — no widgets, live activities, share, safari, notification service, or app clips.

## visionOS
- Use `"visionos"` only when the user explicitly mentions Vision Pro, visionOS, spatial, or Apple Vision.
- Do not set device_family or watch_project_shape for visionos.
- visionOS apps cannot use camera (enterprise only), healthkit, haptics, maps, speech, or app-review.
- visionOS supports widget extensions only — no live activities, share, notification service, safari, or app clips.
- visionOS uses SwiftUI + RealityKit. No UIKit.
- For iPad or universal requests, include adaptive-layout in rule_keys.
- For universal (iPhone+iPad) apps, plan NavigationSplitView as the primary navigation for list-detail flows. Use NavigationStack only for purely linear flows (onboarding, checkout). Mention NavigationSplitView in the components field of the main container view file plan.
- For universal apps, use adaptive grids (GridItem(.adaptive)) and size-class-aware layouts in file purpose/components descriptions.

## macOS
- Use `"macos"` only when the user explicitly mentions Mac, macOS, desktop app, or Mac app.
- Do not set device_family or watch_project_shape for macos.
- macOS apps cannot use healthkit, haptics, or speech.
- macOS supports widget, share, and notification_service extensions — no live activities, app clips, or safari extensions.
- macOS uses SwiftUI natively. No UIKit. AppKit bridge when needed.
- macOS apps should include: Settings scene (Cmd+,), menu bar customization (CommandMenu/CommandGroup), keyboard shortcuts on all actions, and window management (WindowGroup, Window).

## Platform-Specific Design Constraints

### visionOS Design Rules
- AppTheme palette must be **minimal and low-saturation**. Colors are only for accent buttons, badges, and small decorative elements.
- **NEVER use opaque background colors** on windows or main containers. visionOS windows use system glass material.
- Text must use system vibrancy (`.foregroundStyle(.primary)`, `.secondary`, `.tertiary`) — not AppTheme color tokens for body text.
- There is **no dark mode** on visionOS — the glass material auto-adapts. Do not include dark-mode in rule_keys for visionOS-only apps.
- All interactive elements MUST have `.hoverEffect()`.

### tvOS Design Rules
- AppTheme palette must use **muted, desaturated colors** — saturation looks overwhelming on large TV screens.
- tvOS defaults to **dark appearance**. Design accordingly — light text on dark backgrounds.
- Focus is indicated through **scale and shadow**, not color changes. Do not use color to indicate focus state.
- Content imagery is the primary visual element — minimize text, maximize images ("show, don't tell").
- Design for **10-foot viewing distance** — large text, generous spacing (40-80pt between elements).

### macOS Design Rules
- Sidebars should be **translucent** (NavigationSplitView handles this automatically).
- Content areas use opaque AppTheme backgrounds, similar to iOS.
- Every primary action needs a keyboard shortcut. Menu bar customization is expected.
- macOS apps should feel like professional desktop tools, not enlarged phone apps.

## Multi-Platform

When the user requests multiple platforms (e.g., iOS + watchOS + tvOS):

- **Directory structure**: `{AppName}/` for iOS source files, `{AppName}Watch/` for watchOS source files, `{AppName}TV/` for tvOS source files, `{AppName}Vision/` for visionOS source files, `{AppName}Mac/` for macOS source files, `Shared/` for cross-platform code (models, utilities, themes).
- Each platform needs its own `@main` App entry point in its respective directory.
- Set the top-level `platforms` array to list all targeted platforms, e.g. `["ios", "watchos", "tvos", "visionos", "macos"]`.
- Set the `platform` field on each file entry to indicate which platform it belongs to: `"ios"`, `"watchos"`, `"tvos"`, `"visionos"`, `"macos"`, or `""` for shared files.
- Shared code (models, utilities, themes) should have `platform: ""` and live in the `Shared/` directory so all targets can reference it.
