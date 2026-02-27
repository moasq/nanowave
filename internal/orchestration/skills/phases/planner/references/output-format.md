# Output Format

## Contents
- JSON structure
- Field rules
- Available rule_keys

Return ONLY valid PlannerResult JSON with design, files, models, permissions, extensions, localizations, platform, platforms, watch_project_shape, device_family, rule_keys, packages, integrations, and build_order.

## File Entry Fields

Every file entry must include:
- path: relative file path (e.g. "Features/Notes/NoteListView.swift")
- type_name: primary Swift type in this file (required, non-empty)
- purpose: what this file does
- components: key Swift types and signatures as a single string summary (NOT an array)
- data_access: "in-memory", "@AppStorage", "none", etc.
- depends_on: array of file path strings this file imports from (must exist in files array)
- platform: which platform this file belongs to — `"ios"`, `"watchos"`, `"tvos"`, `"visionos"`, `"macos"`, or `""` for shared/cross-platform files

## Extension Entry Fields

Every extension entry MUST include:
- kind: the extension type — REQUIRED, MUST be non-empty. Valid values: `widget`, `live_activity`, `share`, `notification_service`, `safari`, `app_clip`, `tv_top_shelf`
- name: the Xcode target name (e.g. "MyAppWidget")
- purpose: what this extension does

An extension with an empty `kind` will produce a broken Xcode project (invalid bundle ID, missing NSExtensionPointIdentifier). NEVER omit kind.

## Canonical Field Rules

- `files[].components` must be a single string summary (NOT an array).
- `files[].depends_on` must be an array of file path strings.
- `extensions[].kind` MUST be non-empty — it determines the bundle ID suffix and Info.plist configuration.
- `watch_project_shape` values are only `watch_only` or `paired_ios_watch`.
- If `watch_project_shape` is present, `platform` must be `watchos`.
- `platform` values: `ios` (default), `watchos`, `tvos`, `visionos`, `macos`.
- `platforms`: array of platform strings when targeting multiple platforms, e.g. `["ios", "watchos", "tvos"]`. Each element must be `"ios"`, `"watchos"`, `"tvos"`, `"visionos"`, or `"macos"`. Omit or use `[]` for single-platform projects.
- For `tvos`, do not set `device_family` or `watch_project_shape`.
- `build_order`: Models → Theme → ViewModels → Views → App. Respects depends_on.

## Available rule_keys

Include a key if ANY file uses that feature. Design-system, navigation, layout, components, and swiftui are always loaded — do NOT include them.

Features: authentication, notifications, localization, dark-mode, app-review, website-links, haptics, timers, charts, camera, maps, biometrics, healthkit, speech, storage, apple-translation, siri-intents, foundation-models, supabase, repositories

UI refinement: view-complexity, typography, color-contrast, spacing-layout, feedback-states, view-composition, accessibility, gestures, adaptive-layout, liquid-glass, animations

Extensions: widgets, live-activities, share-extension, notification-service, safari-extension, app-clips

## Integrations

When `backend_needs` is present in the analysis, include an `integrations` array listing the backend providers to activate.
Currently available: `"supabase"`.

When integrations includes `"supabase"`:
- Add `"supabase"` to `rule_keys` so the Supabase skill is loaded
- Add the Supabase package: `{"name": "Supabase", "reason": "Backend auth, database, and storage via Supabase Swift SDK"}`
- Plan `AppConfig.swift` with static Supabase URL + anon key constants
- Plan `SupabaseService.swift` as singleton `@Observable` with `SupabaseClient`
- Models use `Codable` (NOT `@Model`) — Supabase is the persistence layer, not SwiftData

When `backend_needs.db` is true:
- **REQUIRED: Populate the `models` array** with every entity that maps to a Supabase table. Each model entry needs `name` (PascalCase), `storage: "Supabase"`, and `properties` array with name/type for each column. These model entries drive automatic SQL table generation — if `models` is empty, no tables will be created and the backend will be left empty.
- Add `"repositories"` to `rule_keys` so the repository pattern skill is loaded
- Plan domain models in `Models/` — conform to `Identifiable` (NOT `Codable`), use Swift enums and URL types
- Plan `Repositories/{Entity}/{Entity}Repository.swift` for each entity — protocol with async/throws methods returning domain models
- Plan `Repositories/{Entity}/Supabase{Entity}Repository.swift` for each entity — concrete implementation containing DTO, insert DTO, `init(dto:)` mapping, and Supabase queries
- Set `data_access: "Supabase"` on repository files, `data_access: "none"` on ViewModels (they use protocol injection)
- ViewModels receive repository protocols via init — never concrete types

Example `models` for a recipe sharing app:
```json
"models": [
  {"name": "Recipe", "storage": "Supabase", "properties": [
    {"name": "id", "type": "UUID"},
    {"name": "userId", "type": "UUID"},
    {"name": "title", "type": "String"},
    {"name": "description", "type": "String"},
    {"name": "imageUrl", "type": "String?"},
    {"name": "createdAt", "type": "Date"}
  ]},
  {"name": "Profile", "storage": "Supabase", "properties": [
    {"name": "id", "type": "UUID"},
    {"name": "username", "type": "String"},
    {"name": "avatarUrl", "type": "String?"}
  ]}
]
```

When `backend_needs.storage` is true:
- Plan `Services/Storage/StorageService.swift` — singleton wrapping Supabase storage with `uploadImage()` (compress + upload + return URL) and `deleteFile()`
- Set `data_access: "Supabase"` on StorageService
- StorageService includes built-in image compression (resize to max 2048px, iterative JPEG quality until under 5 MB)
- ViewModels that upload files use StorageService for file operations and repository protocols for database updates — never call `SupabaseService.shared.client.storage` directly from ViewModels

When `backend_needs.auth` is true:
- Add `"authentication"` to `rule_keys` so the authentication skill is loaded
- Plan `Services/Auth/AuthService.swift` — NOT inside `Features/`
- Plan `Features/Auth/AuthView.swift` + `AuthViewModel.swift`
- Plan `Features/Common/AuthGuardView.swift` (gate mode) or inline auth checks (optional mode)
- When Supabase is also present: AuthService delegates to `SupabaseService.shared.client.auth` for actual auth calls

## Package Entries

**Default is ZERO packages.** Most apps need none. Only add a package when: (1) no native API exists for the capability, or (2) the native approach would require 100+ lines of complex code that the package eliminates. See the workflow reference for the full decision threshold and native-first table.

Each entry has `name` (the package name) and `reason` (what it enables that native code cannot reasonably achieve).

Available curated packages (the build phase resolves exact URLs, versions, and products):

| Category | Available packages |
|---|---|
| Image loading & caching | Kingfisher, Nuke, SDWebImageSwiftUI |
| Animated GIFs | Gifu |
| SVG rendering | SVGView |
| Image editing | Brightroom, CropViewController |
| Audio waveform | DSWaveformImage |
| Audio engine | AudioKit |
| Animations | Lottie |
| Visual effects | ConfettiSwiftUI, Pow, Vortex |
| Shimmer (animated) | Shimmer |
| Loading indicators | ActivityIndicatorView |
| Toasts & popups | PopupView, AlertToast |
| Onboarding | WhatsNewKit, ConcentricOnboarding |
| Calendar UI | HorizonCalendar |
| Chat UI | ExyteChat |
| Flow / wrap layout | SwiftUI-Flow |
| Waterfall / masonry grid | WaterfallGrid |
| Markdown rendering | MarkdownUI |
| Rich text editing | RichTextKit |
| Syntax highlighting | Highlightr |
| QR codes (stylized) | EFQRCode |
| Keychain storage | KeychainSwift, Valet |
| Backend | Supabase |

If a feature needs a package not in this table, include your best guess — the build phase will search the internet and resolve it.

Example — app with a photo grid that loads hundreds of remote images with prefetch and disk caching:
```json
"packages": [
  {"name": "Kingfisher", "reason": "Disk-cached image loading with prefetch and downsampling for photo grid — AsyncImage has no disk cache or prefetch"}
]
```

Example — no packages needed (most apps):
```json
"packages": []
```
