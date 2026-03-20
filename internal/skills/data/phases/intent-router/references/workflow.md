# Workflow

## Contents
- Phase steps
- Stop conditions

1. Read the user request once and detect obvious signals with local rules first.
2. If the request clearly mentions watch, iPad, universal, Apple TV/tvOS, Vision Pro/visionOS/spatial, or Mac/macOS/desktop app, return hints directly.
3. If wording is unclear or conflicting, use the model fallback.
4. Return advisory hints only. The analyzer and planner still follow explicit user wording.
