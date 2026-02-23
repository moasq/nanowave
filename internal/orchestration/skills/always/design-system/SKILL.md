---
name: "design-system"
description: "Design system rules: AppTheme token pattern, Color(hex:) extension, Colors/Spacing/Style enums, SF Symbols, typography tokens. Use when defining colors, spacing, fonts, or any visual design tokens, or when creating/editing AppTheme. Triggers: AppTheme, Color, .primary, .secondary, spacing, cornerRadius, font, SF Symbol."
---
# Design System Rules

## AppTheme Pattern
Every app **MUST** use a centralized theme with **nested enums** for `Colors`, `Fonts`, and `Spacing`. Do NOT use a flat enum with top-level static properties.

```swift
// REQUIRED — always use nested enums
import SwiftUI

enum AppTheme {
    enum Colors {
        static let accent = Color.blue       // one accent per app
        static let textPrimary = Color.primary
        static let textSecondary = Color.secondary
        static let background = Color(.systemBackground)
        static let surface = Color(.secondarySystemBackground)
        static let cardBackground = Color(.secondarySystemGroupedBackground)
    }

    enum Fonts {
        static let largeTitle = Font.largeTitle
        static let title = Font.title
        static let headline = Font.headline
        static let body = Font.body
        static let caption = Font.caption
    }

    enum Spacing {
        static let small: CGFloat = 8
        static let medium: CGFloat = 16
        static let large: CGFloat = 24
        static let cornerRadius: CGFloat = 12
    }
}
```

```swift
// FORBIDDEN — never use flat structure
enum AppTheme {
    static let accentColor = Color.blue   // ❌ wrong
    static let spacing: CGFloat = 8       // ❌ wrong
}
```

Reference as: `AppTheme.Colors.accent`, `AppTheme.Fonts.headline`, `AppTheme.Spacing.medium`

## Typography
- **System fonts only** — use SwiftUI font styles: `.largeTitle`, `.title`, `.headline`, `.body`, `.caption`
- No custom fonts, no downloaded fonts
- Use `AppTheme.Fonts` for consistent sizing

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

```swift
extension Color {
    init(light: String, dark: String) {
        self.init(uiColor: UIColor { traits in
            traits.userInterfaceStyle == .dark ? UIColor(Color(hex: dark)) : UIColor(Color(hex: light))
        })
    }
}
```

## Colors
- **One accent color** that fits the app's purpose
- Use semantic colors: `.primary`, `.secondary`, `Color(.systemBackground)`
- Do NOT add dark mode support, colorScheme checks, or custom dark/light color handling unless the user explicitly requests it
- Use `Color(hex:)` with palette values — NEVER hardcoded SwiftUI colors like `.blue` or `.orange`
- Keep brand/surface tokens explicit in AppTheme so appearance changes do not shift core palette identity

## Spacing Standards
- **16pt** standard padding (outer margins, section spacing)
- **8pt** compact spacing (between related elements)
- **24pt** large spacing (between major sections)
- Use `AppTheme.Spacing` constants throughout

## Empty States
Every list or collection MUST have an empty state. Use `ContentUnavailableView` (iOS 17+) for a polished look:

```swift
// Required — show when collection is empty
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

For custom empty states, use a styled VStack with SF Symbol + descriptive text:

```swift
VStack(spacing: 16) {
    Image(systemName: "tray")
        .font(.system(size: 48))
        .foregroundStyle(.secondary)
    Text("Nothing here yet")
        .font(.title3)
    Text("Add your first item to get started")
        .font(.subheadline)
        .foregroundStyle(.secondary)
}
```

## Surface Materials
Map the design `surfaces` token to SwiftUI materials:
- **glass** → `.ultraThinMaterial` (modern/translucent)
- **material** → `.regularMaterial` (depth/layers)
- **solid** → opaque `Color` from palette (clean/opaque)
- **flat** → no shadows, no materials (minimal)

## Sheet Sizing
Always specify `presentationDetents` on `.sheet`:
- Small option pickers → `.height(N)` (calculate based on content)
- Medium forms → `.medium`
- Complex multi-section → `.large`
- Prefer card-style rows (background, cornerRadius, shadow) over plain List rows
- Horizontal button bars with 4+ items → make scrollable
- Use `.sheet` / `.fullScreenCover` for creation forms

## Animations
Use subtle, purposeful animations for state changes and list mutations:

```swift
// Toggle/complete actions — spring animation
withAnimation(.spring) {
    item.isComplete.toggle()
}

// List insertions/removals — combine opacity + scale
.transition(.opacity.combined(with: .scale))

// Numeric text changes
.contentTransition(.numericText())

// Filter/tab changes
.animation(.default, value: selectedFilter)
```

Rules:
- **Always** use `withAnimation(.spring)` for toggle/complete state changes
- **Always** add `.transition(.opacity.combined(with: .scale))` for list add/remove
- **Never** add gratuitous motion that slows down interaction
- Keep animations subtle — `.spring` and `.default` curves only
