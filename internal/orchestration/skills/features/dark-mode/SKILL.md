---
name: "dark-mode"
description: "Dark mode support: 3-way appearance picker, preferredColorScheme, Color(light:dark:) adaptive tokens, colorScheme environment. Use when adding dark/light mode toggle, creating adaptive colors, or respecting system appearance. Triggers: colorScheme, preferredColorScheme, dark mode, appearance, Color(light:dark:)."
---
# Dark Mode

DARK/LIGHT MODE:
- 3-way picker (system/light/dark) is the standard pattern:
  @AppStorage("appearance") private var appearance: String = "system"
  private var preferredColorScheme: ColorScheme? {
      switch appearance { case "light": return .light; case "dark": return .dark; default: return nil }
  }
  .preferredColorScheme(preferredColorScheme)    // on outermost container in @main app
- CRITICAL: .preferredColorScheme() MUST be in the root @main app, NOT just in the settings view.
- System option: .preferredColorScheme(nil) follows device setting.
- Settings screen: Picker with light/dark/system options writing to @AppStorage("appearance").

ADAPTIVE THEME COLORS (no color assets needed):
- Switch ALL AppTheme palette colors from plain Color(hex:) to Color(light:dark:) with TWO hex values:
  static let background = Color(light: Color(hex: "#F8F9FA"), dark: Color(hex: "#1C1C1E"))
  static let surface = Color(light: Color(hex: "#FFFFFF"), dark: Color(hex: "#2C2C2E"))
- Color(light:dark:) uses UIColor(dynamicProvider:) on iOS/tvOS, NSColor(name:dynamicProvider:) on macOS — reacts to .preferredColorScheme() automatically.
- On macOS, use `#if canImport(AppKit)` with `NSColor` instead of `UIColor` (see design-system skill for full extension).
- YOU decide the dark palette based on app mood — user does not specify dark colors.
- Dark palette guidelines: darken backgrounds (#1C1C1E, #2C2C2E), lighten/brighten accents slightly, use Color.primary/Color.secondary for text.
- AppTheme MUST include the Color(light:dark:) extension (see shared constraints).

PLATFORM RESTRICTIONS:
- **visionOS**: Dark mode is NOT supported. visionOS glass material auto-adapts to the physical environment. Do not use dark-mode rule_key for visionOS apps.
- **macOS**: Use NSColor (AppKit), not UIColor (UIKit). See design-system skill for the macOS-specific Color(light:dark:) extension.
- **Multi-platform**: Use `#if canImport(UIKit)` / `#if canImport(AppKit)` guards for the Color(light:dark:) extension in shared code.
