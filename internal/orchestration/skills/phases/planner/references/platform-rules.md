# Platform Rules

## Exact Values (use these strings verbatim)
- `platform`: exactly `"ios"`, `"watchos"`, or `"tvos"` — no other values
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
- For iPad or universal requests, include adaptive-layout in rule_keys.
- For universal (iPhone+iPad) apps, plan NavigationSplitView as the primary navigation for list-detail flows. Use NavigationStack only for purely linear flows (onboarding, checkout). Mention NavigationSplitView in the components field of the main container view file plan.
- For universal apps, use adaptive grids (GridItem(.adaptive)) and size-class-aware layouts in file purpose/components descriptions.

## Multi-Platform

When the user requests multiple platforms (e.g., iOS + watchOS + tvOS):

- **Directory structure**: `{AppName}/` for iOS source files, `{AppName}Watch/` for watchOS source files, `{AppName}TV/` for tvOS source files, `Shared/` for cross-platform code (models, utilities, themes).
- Each platform needs its own `@main` App entry point in its respective directory.
- Set the top-level `platforms` array to list all targeted platforms, e.g. `["ios", "watchos", "tvos"]`.
- Set the `platform` field on each file entry to indicate which platform it belongs to: `"ios"`, `"watchos"`, `"tvos"`, or `""` for shared files.
- Shared code (models, utilities, themes) should have `platform: ""` and live in the `Shared/` directory so all targets can reference it.
