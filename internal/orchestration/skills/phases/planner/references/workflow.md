# Workflow

## Contents
- Phase steps
- Design palette rules
- Directory structure
- Extension rules
- Validation checklist

## Phase Steps

1. Read the analysis JSON and preserve user intent exactly.
2. Produce a file-level plan with paths, type names, components, data access, dependencies, and build order.
3. Add design tokens and palette choices that respect explicit user styling.
4. Add permissions, extensions, localizations, and rule_keys only when needed.
5. Return JSON only for PlannerResult.

## Design Palette Rules — Every App Must Look Unique

- If the user specifies ANY hex color or named color, use that EXACT value as primary. Do NOT reinterpret or shift.
- If no colors specified, pick a 5-color hex palette that fits the app's domain. NEVER use system blue (#007AFF) as primary.
- Category examples: health → earth tones (#2D6A4F), finance → cool blues (#1B4965), social → vibrant corals (#FF6B6B), food → warm oranges (#E07A5F), productivity → deep purples (#5B21B6), fitness → bold energetics (#EF4444).
- font_design: "rounded" = friendly, "serif" = editorial, "monospaced" = technical, "default" = neutral.
- corner_radius: 20 = bubbly, 16 = friendly, 12 = standard, 8 = sharp/professional.
- density: "spacious" = breathing room, "standard" = balanced, "compact" = data-dense.
- surfaces: "glass" = modern/translucent, "material" = depth, "solid" = clean, "flat" = minimal.
- app_mood: one-word feel (calm, energetic, playful, elegant, bold, cozy, minimal).

## Directory Structure (MANDATORY)

- Models/ → structs with sampleData.
- Theme/ → AppTheme only.
- Config/ → App configuration (AppConfig.swift with API keys, endpoints).
- Features/<Name>/ → View + ViewModel co-located.
- Features/Common/ → shared views/services.
- App/ → @main entry + RootView + MainView (three files minimum).
- NEVER use flat Views/, ViewModels/, or Components/ directories.
- For iPad/universal: MainView MUST use NavigationSplitView for list-detail flows.
- For tvOS: MainView MUST use TabView with top tabs. No NavigationSplitView. Use horizontal shelves for browsing.

## Extension Rules

- Extension source files MUST use paths under Targets/{ExtensionName}/.
- Shared types (e.g. ActivityAttributes) go in Shared/ directory (NOT in Models/).
- Every extension target MUST have a @main entry point file — missing this causes a linker error.
- Siri voice commands use modern App Intents (in-process) — no extension target needed.

## Package Validation

**Default is ZERO packages.** Most apps need none. Every package added is a dependency the user must maintain — only add one when you can justify it passes the threshold below.

Each package entry must have a non-empty `name` and a `reason` explaining what it enables beyond native frameworks.

### Decision threshold — a package is justified ONLY when:

1. **No native API exists** for the capability (e.g. Lottie for After Effects playback, MarkdownUI for Markdown rendering, Highlightr for syntax highlighting, SwiftUI-Flow for wrapping layout, WaterfallGrid for masonry grid). These are clear wins.
2. **Native API exists but the package saves 100+ lines of non-trivial code** (e.g. Kingfisher replaces building a full disk-cache + prefetch + downsampling pipeline on top of URLSession; AudioKit replaces building an audio synthesis engine on top of AVAudioEngine). The code saved must be genuinely complex, not just boilerplate.
3. **The user explicitly names a package.** User intent overrides native-first.

### A package is NOT justified when:

- Native SwiftUI/UIKit handles it in under ~50 lines. Write the native code instead.
- The package only saves trivial boilerplate. A small helper or extension is better than a dependency.
- You are adding it "just in case" or for a minor enhancement. If the core feature works without it, skip it.

### Native-first examples — do NOT use packages for these:

| Feature | Native solution | NOT this package |
|---|---|---|
| Charts | Swift Charts (iOS 16+) | DGCharts |
| Photo picking | PhotosUI / PhotosPicker | — |
| Maps | MapKit | — |
| Simple animations | withAnimation, .animation, PhaseAnimator | — |
| Media playback | AVFoundation, AVKit | — |
| Bottom sheets | .presentationDetents() (iOS 16.4+) | BottomSheet |
| Barcode/QR scanning | DataScannerViewController (VisionKit, iOS 16+) | CodeScanner |
| Plain QR generation | CIQRCodeGenerator (CoreImage) | EFQRCode |
| Static skeleton loading | .redacted(reason: .placeholder) | Shimmer (only if animated shimmer is needed) |
| Basic progress indicators | ProgressView with ProgressViewStyle | ActivityIndicatorView (only if 30+ preset animations needed) |
| In-app web content | SFSafariViewController / WKWebView | — |
| Haptic feedback | UIFeedbackGenerator | — |
| Keychain (simple read/write) | Security framework (small helper) | KeychainSwift (only if multiple keys, biometric access, or shared groups needed) |

### Curated registry

The build phase has a curated registry of pre-validated packages with exact URLs, versions, and integration details. Use the package name from the registry when possible:
- **Images**: Kingfisher, Nuke, SDWebImageSwiftUI
- **GIF/SVG**: Gifu, SVGView
- **Image editing**: Brightroom, CropViewController
- **Audio**: AudioKit, DSWaveformImage
- **Animations**: Lottie
- **Effects**: ConfettiSwiftUI, Pow, Vortex
- **Shimmer/loading**: Shimmer, ActivityIndicatorView
- **Toasts/popups**: PopupView, AlertToast
- **Onboarding**: WhatsNewKit, ConcentricOnboarding
- **Calendar**: HorizonCalendar
- **Chat**: ExyteChat
- **Flow/wrap layout**: SwiftUI-Flow
- **Waterfall/masonry grid**: WaterfallGrid
- **Markdown**: MarkdownUI
- **Rich text**: RichTextKit
- **Syntax highlighting**: Highlightr
- **QR codes (stylized)**: EFQRCode
- **Keychain**: KeychainSwift, Valet

If a feature needs a package not in the registry, include your best guess — the build phase will search the internet and resolve it.

If no packages are needed, use an empty array: `"packages": []`.

## Validation Checklist (verify before returning)

1. All files have ALL mandatory fields (path, type_name, purpose, components, data_access, depends_on) — none empty.
2. All depends_on paths exist in files array; build_order respects dependencies.
3. Views with business logic have a ViewModel; all files under Features/<Name>/ or Features/Common/.
4. Models conform to Identifiable, Hashable, Codable with static sampleData. **When Supabase is active: the `models` JSON array MUST contain every entity that maps to a Supabase table (name, storage: "Supabase", properties). Empty models array = build failure.**
5. System framework usage → matching permission entry; shared service for repeated framework usage.
6. Palette has 5 valid hex colors (#RRGGBB). Primary is NOT #007AFF unless intentional.
7. AppTheme components list Color(hex:) extension, Colors (with textPrimary/textSecondary/textTertiary), Fonts (with plan's fontDesign applied), Spacing, and Style enums.
8. If user specified colors, EXACT hex value appears as design.palette.primary.
9. @AppStorage values written in child views → root App file MUST read and apply them.
10. Extension files under Targets/{ExtensionName}/ with @main entry points.
11. Extension bundle identifiers MUST NOT contain underscores.
12. Models passed across async boundaries must conform to Sendable.
13. Include relevant rule_keys for features used by planned files.
14. iOS 26+ target → include liquid-glass in rule_keys.
15. Any transitions or spring animations → include animations in rule_keys.
