---
name: "ui-review"
description: "Post-build visual UI/UX review: capture simulator screenshots, evaluate with vision, collect findings, fix issues sequentially. Use after successful builds to ensure visual quality."
tags: "review, screenshots, ui, ux, visual, evaluation, quality"
---
# Post-Build UI Review

After a successful build, capture screenshots and evaluate the visual quality of the generated app. This ensures the UI matches expectations before finalization.

## Workflow

### Step 1: Capture Screenshots

Call `nw_capture_screenshots` with the project directory and scheme name. This:
- Boots the iOS simulator
- Builds and installs the app
- Captures the launch screen
- Returns the screenshot path and simulator UDID

### Step 2: Evaluate Each Screenshot

Read each screenshot file to view it with your vision capability. For each screen, evaluate against the checklist in references/evaluation-checklist.

### Step 3: Collect All Findings

Gather all findings from all screenshots into a single list, ordered by severity:
- **Critical** — app is broken, blank screen, crash on launch
- **High** — missing content, broken layout, unreadable text
- **Medium** — AppTheme violations, spacing issues, alignment problems
- **Low** — minor polish (shadows, border radius consistency)

### Step 4: Fix Sequentially

**Fix issues ONE AT A TIME**, rebuilding between each fix. This prevents cascading breakage from simultaneous changes to shared components.

Process for each fix:
1. Identify the file(s) that need changes
2. Make the minimal edit
3. Rebuild with `nw_xcode_build`
4. Recapture with `nw_capture_screenshots` to verify the fix
5. Confirm the fix didn't break other screens
6. Move to the next issue

### Step 5: Final Verification

After all fixes, do one final `nw_capture_screenshots` pass. Read the screenshot and confirm all findings are resolved.

## Capturing Additional Screens

The simulator stays running after `nw_capture_screenshots`. To capture other screens:

```bash
# Navigate to a different tab/screen (if the app supports deep links)
xcrun simctl openurl <UDID> "myapp://settings"

# Capture the current screen
xcrun simctl io <UDID> screenshot /path/to/project/screenshots/review/settings.png
```

For apps without deep links, you cannot navigate programmatically — evaluate only the launch screen.

## When to Skip

Skip the UI review if:
- The build failed (fix compilation first)
- The user explicitly says "don't review" or "ship it"
- This is a quick edit to a single file (not a full build)
