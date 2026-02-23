# Examples

- Apple Watch workout app with iPhone companion -> platform_hint=watchos, watch_project_shape_hint=paired_ios_watch.
- Apple TV streaming app -> platform_hint=tvos, no device_family or watch_project_shape.
- Habit tracker app -> if no explicit platform wording, return low-confidence hints or use fallback.
- App mentioning iPhone, iPad, Watch, and TV -> platform_hint=ios, device_family_hint=universal. Reason should note the user also wants watch and TV versions. NEVER use "multiplatform".

## Multi-Platform Example

User: "Build a focus timer for iPhone, iPad, Apple Watch, and Apple TV"
-> operation: "build", platform_hint: "ios", platform_hints: ["ios", "watchos", "tvos"], device_family_hint: "universal", confidence: 0.95
