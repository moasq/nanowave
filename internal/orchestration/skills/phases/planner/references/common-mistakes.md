# Common Mistakes

## Contents
- Structure mistakes
- Platform mistakes
- Plan quality mistakes

## Structure Mistakes

- Missing mandatory file fields (path, type_name, purpose, components, data_access, depends_on).
- Using flat Views/ or ViewModels/ directories instead of Features/<Name>/.
- Missing App/ directory with three required files (@main App, RootView, MainView).
- Extension source files NOT under Targets/{ExtensionName}/.
- Shared types in Models/ instead of Shared/ (extensions can't see Models/).
- Extension targets missing @main entry point file (causes linker error).
- Extensions with empty `kind` field — this causes broken bundle IDs (trailing dot) and missing NSExtensionPointIdentifier. Every extension MUST have a valid kind.

## Platform Mistakes

- Using watchOS platform while also setting device_family.
- Using tvOS platform while also setting device_family or watch_project_shape.
- Forgetting adaptive-layout in rule_keys for iPad or universal requests.
- Defaulting to iPad or universal — always default to iPhone unless user explicitly says iPad.
- Using non-watchOS features on watchOS (camera, foundation-models, adaptive-layout, liquid-glass).
- Using non-tvOS features on tvOS (camera, biometrics, healthkit, haptics, maps, speech, apple-translation).
- Adding unsupported extensions on tvOS (only tv-top-shelf is supported).

## Plan Quality Mistakes

- Dead @AppStorage values (written but never read at root) — this is a critical bug.
- depends_on paths that don't exist in the files array.
- build_order that doesn't respect dependencies.
- Missing sampleData on model types.
- Empty components field (builder uses this as its sole reference).
- Extension bundle identifiers containing underscores (invalid in UTI).
- Using system blue (#007AFF) as primary unless intentional.
- Not including liquid-glass in rule_keys for iOS 26+ apps.

## Multi-Platform Mistakes

- Forgetting to set the top-level `platforms` array when multiple platforms are requested.
- Not setting the `platform` field on file entries — every file must declare which platform it belongs to (or `""` for shared).
- Putting platform-specific code in `Shared/` — only truly cross-platform models, utilities, and themes belong there.
