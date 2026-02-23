# Workflow

## Contents
- Phase steps
- Investigation strategy
- Stop conditions

## Phase Steps

1. Run the build command and read the error output.
2. Group errors by root cause — many errors often stem from one broken file.
3. Read the failing source files before attempting fixes.
4. Fix the root cause first (see error-triage.md for priority order).
5. Rebuild after each fix batch.
6. Repeat until the build is green.

## Investigation Strategy

1. READ error messages carefully — identify the error type and file.
2. INVESTIGATE before fixing — read the related source files to understand context.
3. FIX based on evidence — never guess at API signatures or type names.
4. When unsure about an Apple API, use search_apple_docs or get_apple_doc_content.

## XcodeGen Tools

If the error is a project configuration issue (missing target, wrong setting), use xcodegen MCP tools:
- add_permission, add_extension, add_entitlement, set_build_setting, regenerate_project.
NEVER manually edit project.yml.

## Stop Conditions

- The xcodebuild command succeeds with zero errors.
- Do NOT stop after reducing errors — zero is the target.
- If a hook emits a warning but the build compiles, the build is green.
