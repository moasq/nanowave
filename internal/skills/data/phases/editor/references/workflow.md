# Workflow

## Contents
- Phase steps
- Stop conditions

1. Read the affected files before editing.
2. Preserve existing working behavior unless the request changes it.
3. Make the requested changes with minimal blast radius.
4. Run a build and fix any new issues.
5. Keep architecture and root wiring intact.

## Adding a New Platform

1. Create the platform source directory (e.g., `{AppName}TV/` for tvOS, `{AppName}Watch/` for watchOS).
2. Write the `@main` App entry point for the new platform in that directory.
3. Use xcodegen MCP tools to add the new target and configure its sources.
4. Build all schemes to verify that existing targets still compile and the new target builds cleanly.
