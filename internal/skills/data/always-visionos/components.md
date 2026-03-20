---
name: "components"
description: "visionOS UI components: glass backgrounds, hover effects, ornaments, 3D content views. Use when working on visionOS component patterns, spatial UI, or Vision Pro interactions. Triggers: Button, glass, hover, ornament, RealityView, Model3D, component."
---
# Component Patterns (visionOS)

## CRITICAL — Glass-First, Minimal Color

visionOS windows use system glass material by default. The glass auto-adapts brightness based on the physical environment. Custom opaque backgrounds destroy this adaptive behavior.

**BANNED on visionOS:**
- Opaque/solid background colors on windows or main containers (`.background(AppTheme.Colors.background)`)
- Colored text (`.foregroundStyle(AppTheme.Colors.primary)` on body text) — use white/system text on glass
- Heavy color saturation — colors may clash with the user's physical environment
- Dark theme backgrounds — there is no dark mode on visionOS, the glass handles it
- `.glassBackgroundEffect()` on list row backgrounds — creates glass-on-glass, text becomes invisible
- `.scrollContentBackground(.hidden)` on Lists — removes contrast needed for text visibility

**REQUIRED on visionOS:**
- Use `.glassBackgroundEffect()` for custom container backgrounds (sidebars, cards, panels)
- Use white or system-default text on glass for readability
- Limit accent color to primary action buttons and key interactive elements only
- Use system vibrancy for text hierarchy (`.primary`, `.secondary`, `.tertiary`)

```swift
// CORRECT — glass container, system text
VStack {
    Text("Settings")
        .font(AppTheme.Fonts.title)
        .foregroundStyle(.primary)      // system vibrancy — auto-adapts
    Text("Configure your preferences")
        .font(AppTheme.Fonts.body)
        .foregroundStyle(.secondary)    // dimmer vibrancy level
}
.padding(AppTheme.Spacing.lg)
.glassBackgroundEffect()

// BANNED — opaque colored background
VStack { content }
    .background(AppTheme.Colors.background) // NO — destroys glass
    .foregroundStyle(AppTheme.Colors.primary) // NO — colored body text
```

## AppTheme Color Usage on visionOS

AppTheme colors are ONLY for:
- `.borderedProminent` button tint (accent color)
- Badge backgrounds (status indicators)
- Small decorative accents (icons, dividers)
- Chart/data visualization elements

AppTheme colors must NEVER be used for:
- Window or view backgrounds
- Card/container backgrounds (use `.glassBackgroundEffect()`)
- Body text color (use `.foregroundStyle(.primary)` / `.secondary` / `.tertiary`)

## Hover Effects
ALL interactive elements MUST have `.hoverEffect()` for eye tracking feedback:
```swift
Button("Play") {
    play()
}
.hoverEffect()

// Custom hover effect
Button { } label: {
    Image(systemName: "star")
        .font(AppTheme.Fonts.title)
}
.hoverEffect(.highlight)
```

## Button Styles
```swift
// Bordered prominent (primary action — accent color tint is appropriate here)
Button("Start", systemImage: "play.fill") {
    start()
}
.buttonStyle(.borderedProminent)
.buttonBorderShape(.capsule)
.hoverEffect()

// Bordered (secondary action)
Button("Settings", systemImage: "gear") {
    openSettings()
}
.buttonStyle(.bordered)
.buttonBorderShape(.roundedRectangle)
.hoverEffect()
```

BUTTON HIERARCHY:
| Level | Style | Use Case |
|-------|-------|----------|
| Primary action | `.borderedProminent` | Start, Play, Confirm |
| Secondary | `.bordered` | Settings, More Info |
| Tertiary | `.borderless` | Dismiss, Cancel |

## Ornaments
Ornaments attach supplementary controls to windows:
```swift
.ornament(attachmentAnchor: .scene(.bottom)) {
    HStack(spacing: 20) {
        Button("Previous", systemImage: "backward.fill") {
            previous()
        }
        Button("Play", systemImage: "play.fill") {
            play()
        }
        Button("Next", systemImage: "forward.fill") {
            next()
        }
    }
    .padding()
    .glassBackgroundEffect()
}
```

## 3D Content — RealityView
For interactive 3D content:
```swift
import RealityKit

RealityView { content in
    if let entity = try? await Entity(named: "Scene", in: realityKitContentBundle) {
        content.add(entity)
    }
}
```

## 3D Content — Model3D
For simple 3D model display (no interaction):
```swift
import RealityKit

Model3D(named: "Globe") { model in
    model
        .resizable()
        .scaledToFit()
} placeholder: {
    ProgressView()
}
```

## Lists
visionOS List views have built-in glass-compatible styling. Do NOT add custom glass backgrounds to list rows — this creates glass-on-glass rendering where text becomes invisible.

```swift
// CORRECT — let the system handle list row backgrounds
List {
    Section("General") {
        NavigationLink("Profile") { ProfileView() }
        Toggle("Notifications", isOn: $notifications)
    }
}
.listStyle(.insetGrouped)
```

```swift
// BANNED — glass on list rows causes invisible text
List {
    ForEach(items) { item in
        ItemRow(item: item)
            .listRowBackground(                         // NO
                RoundedRectangle(cornerRadius: 12)
                    .fill(.clear)
                    .glassBackgroundEffect()             // glass-on-glass = invisible text
            )
    }
}
.scrollContentBackground(.hidden)                       // NO — removes default list contrast
```

**Key rules for Lists on visionOS:**
- Use `.listStyle(.insetGrouped)` — it provides proper visionOS styling
- NEVER use `.scrollContentBackground(.hidden)` — it removes the contrast needed for text visibility
- NEVER apply `.glassBackgroundEffect()` to `.listRowBackground()` — glass-on-glass makes text invisible
- NEVER use `.listRowBackground()` with custom glass shapes — the system handles row backgrounds

## Empty States
```swift
ContentUnavailableView(
    "No Results",
    systemImage: "magnifyingglass",
    description: Text("Try a different search term")
)
```

## NOT Available on visionOS
- No `UIScreen.main.bounds` — use GeometryReader or window sizing
- No UIKit views directly — use SwiftUI
- No haptic feedback (CoreHaptics)
- No camera access (enterprise only)
- No HealthKit
- No dark mode — glass auto-adapts to environment

## Rules
1. **NEVER use opaque background colors on windows or main containers** — use glass
2. Use `.glassBackgroundEffect()` for custom container backgrounds (sidebars, cards, panels) — but **NEVER on list rows**
3. **NEVER use `.scrollContentBackground(.hidden)` or `.listRowBackground()` with glass** — creates invisible text
4. Use `.listStyle(.insetGrouped)` for Lists — system handles row styling
5. ALL interactive elements MUST have `.hoverEffect()` for eye tracking
6. Use system vibrancy (`.primary`, `.secondary`, `.tertiary`) for text — not AppTheme color tokens
7. Limit AppTheme accent colors to buttons and small decorative elements only
8. Use `.buttonBorderShape()` for spatial-appropriate button shapes
9. Use RealityView for interactive 3D, Model3D for display-only 3D
10. Ornaments for supplementary controls attached to windows
11. ONE `.borderedProminent` per visible area
12. Minimum touch target: 60x60 points for comfortable spatial interaction
