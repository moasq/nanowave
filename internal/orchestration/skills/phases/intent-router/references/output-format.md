# Output Format

Return JSON only with: operation, platform_hint, platform_hints, device_family_hint, watch_project_shape_hint, confidence, reason, used_llm.
Use empty strings for unknown hints. confidence must be between 0.0 and 1.0.

## Exact Values (use these strings verbatim)
- `operation`: exactly `"build"`, `"edit"`, or `"fix"` — no other values
- `platform_hint`: exactly `"ios"`, `"watchos"`, `"tvos"`, `"visionos"`, `"macos"`, or `""` — no other values (primary platform for backward compat)
- `platform_hints`: array of platform strings, e.g. `["ios", "watchos", "tvos", "visionos", "macos"]`. Each element must be `"ios"`, `"watchos"`, `"tvos"`, `"visionos"`, or `"macos"`. Use `[]` when only one platform is targeted.
- `device_family_hint`: exactly `"iphone"`, `"ipad"`, `"universal"`, or `""` — no other values
- `watch_project_shape_hint`: exactly `"watch_only"`, `"paired_ios_watch"`, or `""` — no other values

NEVER return compound values like "multiplatform", "multi_platform", "all", "create_app", etc.
If the user mentions multiple platforms, set `platform_hint` to the PRIMARY platform (usually "ios") and list ALL requested platforms in `platform_hints`.
