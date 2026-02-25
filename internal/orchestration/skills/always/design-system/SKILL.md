---
name: "design-system"
description: "Design system rules: AppTheme token pattern, Color(hex:) extension, Colors/Fonts/Spacing/Style enums, SF Symbols, typography tokens. Use when defining colors, spacing, fonts, or any visual design tokens, or when creating/editing AppTheme. Triggers: AppTheme, Color, .primary, .secondary, spacing, cornerRadius, font, SF Symbol."
---
# Design System Rules

## MANDATORY — AppTheme Is the Single Source of Truth

Every color, font, and spacing value in the app MUST come from `AppTheme`. No exceptions.

Before writing ANY view code, verify:
1. Does `AppTheme.Colors` have a token for the color I need? If not, add one.
2. Does `AppTheme.Fonts` have a token for the font I need? If not, add one.
3. Am I using `AppTheme.Spacing` for padding/spacing? If not, switch to it.

## FORBIDDEN — Hardcoded Styling (Zero Tolerance)

These patterns are **BANNED** everywhere in feature views. Violations MUST be caught and fixed.

```swift
// BANNED — hardcoded colors
.foregroundStyle(.white)                    // use AppTheme.Colors.textPrimary
.foregroundStyle(.white.opacity(0.8))       // use AppTheme.Colors.textSecondary
.foregroundStyle(.white.opacity(0.6))       // use AppTheme.Colors.textTertiary
.foregroundStyle(.black)                    // use AppTheme.Colors.textPrimary (or appropriate token)
.foregroundStyle(Color.red)                 // define AppTheme.Colors.error or semantic token
.foregroundStyle(Color.blue)                // define AppTheme.Colors.accent or semantic token
.background(.blue)                          // use AppTheme.Colors.* token
.background(Color(hex: "FF0000"))           // define in AppTheme.Colors, reference the token
.tint(.white)                               // use AppTheme.Colors.* token

// BANNED — hardcoded fonts
.font(.system(size: 48))                    // use AppTheme.Fonts.* token
.font(.system(size: 64))                    // use AppTheme.Fonts.* token
.font(.system(.largeTitle, design: .rounded, weight: .bold))  // define in AppTheme.Fonts
.font(.title2)                              // use AppTheme.Fonts.title2
.font(.caption)                             // use AppTheme.Fonts.caption
.font(.headline)                            // use AppTheme.Fonts.headline

// BANNED — hardcoded spacing
.padding(20)                                // use AppTheme.Spacing.*
.padding(.horizontal, 12)                   // use AppTheme.Spacing.*
VStack(spacing: 10)                         // use AppTheme.Spacing.*
```

```swift
// CORRECT — always use AppTheme tokens
.foregroundStyle(AppTheme.Colors.textPrimary)
.foregroundStyle(AppTheme.Colors.textSecondary)
.font(AppTheme.Fonts.title2)
.font(AppTheme.Fonts.caption)
.padding(AppTheme.Spacing.md)
.padding(.horizontal, AppTheme.Spacing.sm)
VStack(spacing: AppTheme.Spacing.sm)
.background(AppTheme.Colors.surface)
```

## AppTheme Pattern

Every app **MUST** use a centralized theme with **nested enums** for `Colors`, `Fonts`, and `Spacing`. Do NOT use a flat enum with top-level static properties.

```swift
// REQUIRED — always use nested enums
import SwiftUI

enum AppTheme {
    enum Colors {
        static let primary = Color(hex: "...")
        static let secondary = Color(hex: "...")
        static let accent = Color(hex: "...")
        static let background = Color(hex: "...")
        static let surface = Color(hex: "...")

        // Text colors — REQUIRED for every app
        static let textPrimary = Color.white           // or Color.primary for light bg apps
        static let textSecondary = Color.white.opacity(0.8)
        static let textTertiary = Color.white.opacity(0.6)
    }

    enum Fonts {
        static let largeTitle = Font.system(.largeTitle, design: .rounded, weight: .bold)
        static let title = Font.system(.title, design: .rounded, weight: .bold)
        static let title2 = Font.system(.title2, design: .rounded, weight: .semibold)
        static let title3 = Font.system(.title3, design: .rounded, weight: .semibold)
        static let headline = Font.system(.headline, design: .rounded)
        static let body = Font.system(.body, design: .rounded)
        static let callout = Font.system(.callout, design: .rounded)
        static let subheadline = Font.system(.subheadline, design: .rounded)
        static let footnote = Font.system(.footnote, design: .rounded)
        static let caption = Font.system(.caption, design: .rounded)
        static let caption2 = Font.system(.caption2, design: .rounded)
    }

    enum Spacing {
        static let xs: CGFloat = 4
        static let sm: CGFloat = 8
        static let md: CGFloat = 16
        static let lg: CGFloat = 24
        static let xl: CGFloat = 40
    }

    enum Style {
        static let cornerRadius: CGFloat = 12
        static let cardCornerRadius: CGFloat = 16
    }
}
```

```swift
// FORBIDDEN — never use flat structure
enum AppTheme {
    static let accentColor = Color.blue   // wrong
    static let spacing: CGFloat = 8       // wrong
}
```

Reference as: `AppTheme.Colors.accent`, `AppTheme.Fonts.headline`, `AppTheme.Spacing.md`

## Fonts — AppTheme.Fonts Required

The `Fonts` enum MUST exist in every AppTheme. It defines the app's typography tokens using the font design from the plan.

Rules:
- **System fonts only** — use SwiftUI font styles: `.largeTitle`, `.title`, `.headline`, `.body`, `.caption`
- Apply the plan's `fontDesign` (rounded, serif, monospaced, default) via `Font.system(.style, design: .rounded)`
- **NEVER** use raw `.font(.title2)` or `.font(.headline)` in views — always `AppTheme.Fonts.title2`
- **NEVER** use `.font(.system(size: N))` — it opts out of Dynamic Type
- No custom fonts, no downloaded fonts

## Text Colors — AppTheme.Colors.textPrimary Required

Every AppTheme MUST define text color tokens. Views MUST use these instead of `.white`, `.black`, or `.primary`:

| Instead of | Use |
|---|---|
| `.foregroundStyle(.white)` | `AppTheme.Colors.textPrimary` |
| `.foregroundStyle(.white.opacity(0.8))` | `AppTheme.Colors.textSecondary` |
| `.foregroundStyle(.white.opacity(0.6))` | `AppTheme.Colors.textTertiary` |
| `.foregroundStyle(.secondary)` | `AppTheme.Colors.textSecondary` |

## Icons (SF Symbols)
- **SF Symbols only** for all icons — required for every list row, button, empty state, and tab
- Reference via `Image(systemName: "symbol.name")`
- Pick domain-appropriate symbols (e.g. "checkmark.circle.fill" for todos, "note.text" for notes, "heart.fill" for favorites)
- Use `.symbolRenderingMode(.hierarchical)` or `.symbolRenderingMode(.palette)` for visual depth
- No custom icon assets unless the app concept specifically requires them

## Color(hex:) Extension
Every app MUST define a `Color(hex:)` initializer in AppTheme.swift so palette hex values can be used:

```swift
extension Color {
    init(hex: String) {
        let hex = hex.trimmingCharacters(in: .init(charactersIn: "#"))
        let scanner = Scanner(string: hex)
        var rgbValue: UInt64 = 0
        scanner.scanHexInt64(&rgbValue)
        self.init(
            red: Double((rgbValue & 0xFF0000) >> 16) / 255.0,
            green: Double((rgbValue & 0x00FF00) >> 8) / 255.0,
            blue: Double(rgbValue & 0x0000FF) / 255.0
        )
    }
}
```

When the app has appearance switching (dark/light/system), also define:

**iOS / tvOS / visionOS** (UIKit available):
```swift
extension Color {
    init(light: String, dark: String) {
        self.init(uiColor: UIColor { traits in
            traits.userInterfaceStyle == .dark ? UIColor(Color(hex: dark)) : UIColor(Color(hex: light))
        })
    }
}
```

**macOS** (AppKit, no UIKit):
```swift
extension Color {
    init(light: String, dark: String) {
        self.init(nsColor: NSColor(name: nil, dynamicProvider: { appearance in
            let isDark = appearance.bestMatch(from: [.darkAqua, .aqua]) == .darkAqua
            return isDark ? NSColor(Color(hex: dark)) : NSColor(Color(hex: light))
        }))
    }
}
```

**Multi-platform shared code** — use `#if canImport`:
```swift
extension Color {
    init(light: String, dark: String) {
        #if canImport(UIKit)
        self.init(uiColor: UIColor { traits in
            traits.userInterfaceStyle == .dark ? UIColor(Color(hex: dark)) : UIColor(Color(hex: light))
        })
        #elseif canImport(AppKit)
        self.init(nsColor: NSColor(name: nil, dynamicProvider: { appearance in
            let isDark = appearance.bestMatch(from: [.darkAqua, .aqua]) == .darkAqua
            return isDark ? NSColor(Color(hex: dark)) : NSColor(Color(hex: light))
        }))
        #endif
    }
}
```

## Appearance Mode — Single-Appearance Lock via Info.plist

When the app does NOT support both light and dark appearances (no `dark-mode` rule key), the pipeline automatically locks appearance via Info.plist:

- **iOS / tvOS**: `UIUserInterfaceStyle` is set to `Light` in the generated Info.plist (via `INFOPLIST_KEY_UIUserInterfaceStyle` build setting).
- **visionOS**: No appearance lock needed — visionOS has no dark mode. The glass material auto-adapts to the physical environment.
- **macOS**: No appearance lock. macOS apps always follow the system appearance (dark/light). Users expect Mac apps to respect their system preference.

This is handled at the XcodeGen project generation level — **do NOT use `.preferredColorScheme()` for this purpose**. The Info.plist approach ensures the entire app (including system chrome, alerts, and sheets) respects the locked appearance, not just SwiftUI views.

**When the app supports dark mode** (`dark-mode` in rule_keys with `Color(light:dark:)` adaptive tokens), the pipeline omits these keys and the app follows the system appearance.

## Colors
- **One accent color** that fits the app's purpose
- Use semantic colors via `AppTheme.Colors.*` tokens — never raw SwiftUI colors
- Do NOT add dark mode support, colorScheme checks, or custom dark/light color handling unless the user explicitly requests it
- Use `Color(hex:)` with palette values — NEVER hardcoded SwiftUI colors like `.blue` or `.orange`
- Keep brand/surface tokens explicit in AppTheme so appearance changes do not shift core palette identity

### Platform-Specific Color Rules
- **visionOS**: AppTheme colors are ONLY for accent buttons, badges, and small decorative elements. NEVER use AppTheme.Colors for backgrounds or body text. Use system glass and vibrancy instead.
- **tvOS**: AppTheme palette must use muted, desaturated colors. Saturation is overwhelming on large TV screens. Dark-first design — light text on dark backgrounds.
- **macOS**: Standard color usage similar to iOS. Sidebar icons should be monochrome (system handles tinting).
- **iOS**: Full AppTheme color palette usage is appropriate.

## Spacing Standards
- **16pt** standard padding (outer margins, section spacing)
- **8pt** compact spacing (between related elements)
- **24pt** large spacing (between major sections)
- Use `AppTheme.Spacing` constants throughout — never raw numeric values

## Empty States
Every list or collection MUST have an empty state. Use `ContentUnavailableView` (iOS 17+) for a polished look:

```swift
if items.isEmpty {
    ContentUnavailableView(
        "No Notes Yet",
        systemImage: "note.text",
        description: Text("Tap + to create your first note")
    )
} else {
    // Show the list
}
```

## Surface Materials
Map the design `surfaces` token to SwiftUI materials:
- **glass** -> `.ultraThinMaterial` (modern/translucent)
- **material** -> `.regularMaterial` (depth/layers)
- **solid** -> opaque `Color` from palette (clean/opaque)
- **flat** -> no shadows, no materials (minimal)

## Sheet Sizing
Always specify `presentationDetents` on `.sheet`:
- Small option pickers -> `.height(N)` (calculate based on content)
- Medium forms -> `.medium`
- Complex multi-section -> `.large`
- Use `.sheet` / `.fullScreenCover` for creation forms

## Animations
Use subtle, purposeful animations for state changes and list mutations:

```swift
withAnimation(.spring) {
    item.isComplete.toggle()
}

.transition(.opacity.combined(with: .scale))
.contentTransition(.numericText())
.animation(.default, value: selectedFilter)
```

Rules:
- **Always** use `withAnimation(.spring)` for toggle/complete state changes
- **Always** add `.transition(.opacity.combined(with: .scale))` for list add/remove
- **Never** add gratuitous motion that slows down interaction
- Keep animations subtle — `.spring` and `.default` curves only
