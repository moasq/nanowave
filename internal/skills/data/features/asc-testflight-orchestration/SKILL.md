---
name: asc-testflight-orchestration
description: End-to-end release workflows for TestFlight and App Store including build, archive, export, upload, and distribution. Use when asked to publish, submit, distribute to TestFlight, or submit to App Store.
---

# Release and TestFlight Workflow

## Quick path: Adding testers to an existing build

If the user only asks to add/invite a beta tester (NOT build + upload), skip the full workflow. Follow these steps **in exact order**:

```bash
# 1. Check if there are any builds available
asc builds list --app "APP_ID" --limit 3 --output json
```

**If NO builds exist:** Tell the user they need to upload a build first. Offer to run the full build+upload workflow (Step 1-9 below). Do NOT add testers without a build.

**If builds exist:** Continue with external testing below.

### Adding a beta tester (external testing)

External testing works for any email — the tester does NOT need to be an App Store Connect team member. However, external testing requires **Beta App Review** before the tester can install.

```bash
# 2. List existing external groups, or create one
asc testflight beta-groups list --app "APP_ID" --output json
asc testflight beta-groups create --app "APP_ID" --name "Beta Testers"

# 3. Assign the latest build to the external group
asc builds add-groups --build "BUILD_ID" --group "GROUP_ID"

# 4. Add the tester to the group (works for any email)
asc testflight beta-testers add --app "APP_ID" --email "tester@example.com" --group "Beta Testers"
```

**Now you MUST ask the user for confirmation before submitting for Beta App Review.** Say:

> "I've added tester@example.com to the Beta Testers group. To complete the invitation, this build needs to be submitted for **Beta App Review** (Apple reviews it, typically < 24 hours). Once approved, the tester will automatically receive a TestFlight invitation email. Shall I submit for Beta App Review?"

Wait for the user's response. Only proceed if they confirm:

```bash
# 5. Submit for Beta App Review (only after user confirms)
asc testflight review submit --build "BUILD_ID" --confirm

# 6. Check review status
asc testflight review submissions list --build "BUILD_ID"
```

**Do NOT call `asc testflight beta-testers invite` for external testers** — it will fail with "Tester has no installable build" until Beta App Review is approved. Once approved, the tester automatically receives the invitation.

### Internal testing (advanced — team members only)

Internal testing is only for people who are already App Store Connect team members. Internal groups auto-receive all builds and testers get instant access (no review). Use this only when the user explicitly asks for internal testing or the tester is a known team member.

```bash
asc testflight beta-groups create --app "APP_ID" --name "Internal Testers" --internal
asc testflight beta-testers add --app "APP_ID" --email "tester@example.com" --group "Internal Testers"
asc testflight beta-testers invite --app "APP_ID" --email "tester@example.com"
```

Do NOT use `asc builds add-groups` for internal groups — they auto-receive all processed builds.

## Preconditions

- **You MUST build the IPA first** using `xcodebuild archive` + `xcodebuild -exportArchive` (see asc-xcode-build skill).
- Ensure credentials are set (`asc auth login` or `ASC_*` env vars).
- Use a new build number for each upload.
- Prefer `ASC_APP_ID` or pass `--app` explicitly.
- Build must have encryption compliance resolved (see asc-submission-health skill).

## Preferred end-to-end commands

- TestFlight:
  - `asc publish testflight --app <APP_ID> --ipa <PATH> --group <GROUP_ID>[,<GROUP_ID>]`
  - Optional: `--wait`, `--notify`, `--platform`, `--poll-interval`, `--timeout`
- App Store:
  - `asc publish appstore --app <APP_ID> --ipa <PATH> --version <VERSION>`
  - Optional: `--wait`, `--submit --confirm`, `--platform`, `--poll-interval`, `--timeout`

## Full build + upload + distribute workflow

This is a MANDATORY step-by-step workflow. When the user asks to publish/build/upload to TestFlight, you MUST execute ALL steps below in order. Do NOT skip any step. Do NOT claim the app is "already on TestFlight" without verifying. Do NOT stop after planning — execute the commands.

## Step 1: Find and regenerate the Xcode project

```bash
# Find .xcodeproj or .xcworkspace
find . -maxdepth 2 -name "*.xcworkspace" -o -name "*.xcodeproj" | head -5
```

If a `project.yml` exists (XcodeGen project), **always** regenerate the .xcodeproj first:
```bash
xcodegen generate
```
This ensures all build settings (CFBundleIconName, icon assets, etc.) are current in the .xcodeproj.

If a `.xcworkspace` exists, use `-workspace` flag. Otherwise use `-project` flag.

## Step 2: Determine the scheme

```bash
# List available schemes
xcodebuild -list
```

Pick the main app scheme (not tests, not UI tests).

## Step 3: Check current build number

```bash
# Get current build settings
xcodebuild -showBuildSettings -scheme "SCHEME" | grep -E "CURRENT_PROJECT_VERSION|MARKETING_VERSION"
```

Also check the latest build on ASC:
```bash
asc builds list --app "APP_ID" --limit 3 --output json
```

If the local build number is not higher than the latest uploaded build, increment it:
```bash
# Find the project.pbxproj or xcconfig and update CURRENT_PROJECT_VERSION
```

## Step 4: Add ITSAppUsesNonExemptEncryption to Info.plist

Check if `ITSAppUsesNonExemptEncryption` is set. If not, add it to avoid encryption compliance issues:
```bash
# Check in Info.plist or build settings
grep -r "ITSAppUsesNonExemptEncryption" . --include="*.plist" --include="*.pbxproj"
```

If missing, add `ITSAppUsesNonExemptEncryption = NO` to the app's Info.plist.

## Step 5: Clean and archive

```bash
xcodebuild clean archive \
  -scheme "SCHEME" \
  -configuration Release \
  -archivePath /tmp/APP_NAME.xcarchive \
  -destination "generic/platform=iOS" \
  -allowProvisioningUpdates \
  AUTHENTICATION_KEY_PATH AUTH_FLAGS_IF_AVAILABLE
```

If API key authentication flags are provided in the system prompt, include them:
- `-authenticationKeyPath <path>`
- `-authenticationKeyID <id>`
- `-authenticationKeyIssuerID <issuer>`

This step MUST succeed. If it fails, diagnose the error and fix it before proceeding.

## Step 6: Create ExportOptions.plist

```bash
cat > /tmp/ExportOptions.plist << 'PLIST'
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
    <key>method</key>
    <string>app-store-connect</string>
    <key>destination</key>
    <string>upload</string>
</dict>
</plist>
PLIST
```

If you know the team ID, add it:
```xml
<key>teamID</key>
<string>TEAM_ID</string>
```

## Step 7: Export IPA

```bash
xcodebuild -exportArchive \
  -archivePath /tmp/APP_NAME.xcarchive \
  -exportPath /tmp/APP_NAME_Export \
  -exportOptionsPlist /tmp/ExportOptions.plist \
  -allowProvisioningUpdates \
  AUTHENTICATION_KEY_PATH AUTH_FLAGS_IF_AVAILABLE
```

This step MUST succeed. The IPA file will be at `/tmp/APP_NAME_Export/APP_NAME.ipa`.

## Step 8: Upload to App Store Connect

```bash
asc builds upload --app "APP_ID" --ipa "/tmp/APP_NAME_Export/APP_NAME.ipa"
```

Wait for the upload to complete.

## Step 9: Wait for build processing

After upload, the build needs time to process on Apple's servers:
```bash
# Check if the build is processed
asc builds latest --app "APP_ID" --output json
```

If the build is still processing, wait and check again. Use `asc builds info --build BUILD_ID` to check status.

## Step 10: Distribute to TestFlight

### Default: External testing (works for any email)

```bash
# 1. Create an external group and assign the build
asc testflight beta-groups create --app "APP_ID" --name "Beta Testers"
asc builds add-groups --build "BUILD_ID" --group "GROUP_ID"

# 2. Add tester to the group (works for any email)
asc testflight beta-testers add --app "APP_ID" --email "tester@example.com" --group "Beta Testers"
```

**Now ask the user for confirmation before submitting for Beta App Review:**

> "I've added the tester to the Beta Testers group. To complete the invitation, this build needs to be submitted for Beta App Review (typically < 24h). Once approved, the tester will automatically receive a TestFlight invitation. Shall I submit?"

```bash
# 3. Submit for Beta App Review (only after user confirms)
asc testflight review submit --build "BUILD_ID" --confirm

# 4. Check submission status
asc testflight review submissions list --build "BUILD_ID"
```

**Do NOT call `beta-testers invite` for external testers** — it fails with "Tester has no installable build" until review is approved. Once approved, testers automatically receive the TestFlight invitation.

## Step 11: Verify and report

```bash
# Verify the build is on TestFlight
asc builds info --build "BUILD_ID" --output json
```

Report to the user:
- Build version and number
- Upload status
- TestFlight group(s) assigned
- Any warnings or issues

## App Store submission (after upload)

For App Store (not TestFlight), attach the build to a version and submit:
```bash
# Attach build to version
asc versions attach-build --version-id <VERSION_ID> --build <BUILD_ID>

# Submit for review
asc submit create --app <APP_ID> --version <VERSION> --build <BUILD_ID> --confirm

# Check or cancel submission
asc submit status --id <SUBMISSION_ID>
asc submit cancel --id <SUBMISSION_ID> --confirm
```

## macOS Release

macOS apps are distributed as `.pkg` files, not `.ipa`.

### Upload PKG
```bash
asc builds upload \
  --app <APP_ID> \
  --pkg <PATH_TO_PKG> \
  --version <VERSION> \
  --build-number <BUILD_NUMBER> \
  --wait
```

Notes:
- `--pkg` automatically sets platform to `MAC_OS`.
- For macOS `.pkg`, use `asc builds upload --pkg` + attach/submit steps.

### Attach and Submit (macOS)
Same as iOS flow:
```bash
asc builds list --app <APP_ID> --limit 5
asc versions attach-build --version-id <VERSION_ID> --build <BUILD_ID>
asc review submissions-create --app <APP_ID> --platform MAC_OS
asc review items-add --submission <SUBMISSION_ID> --item-type appStoreVersions --item-id <VERSION_ID>
asc review submissions-submit --id <SUBMISSION_ID> --confirm
```

## visionOS / tvOS Release

Same as iOS flow, use appropriate `--platform`: `VISION_OS` or `TV_OS`.

## Pre-submission Checklist

Before submitting to App Store, verify:
- [ ] Build status is `VALID` (not processing)
- [ ] Encryption compliance resolved
- [ ] Content rights declaration set
- [ ] Copyright field populated
- [ ] All localizations complete
- [ ] Screenshots present

See `asc-submission-health` skill for detailed preflight checks.

## Manage groups and testers

After distribution, these commands manage TestFlight access:

- Groups:
  - `asc testflight beta-groups list --app "APP_ID" --paginate`
  - `asc testflight beta-groups create --app "APP_ID" --name "Beta Testers"` (external, default)
  - `asc testflight beta-groups create --app "APP_ID" --name "Internal Testers" --internal` (team members only)
- Testers:
  - `asc testflight beta-testers list --app "APP_ID" --paginate`
  - `asc testflight beta-testers add --app "APP_ID" --email "tester@example.com" --group "Beta Testers"`
  - `asc testflight beta-testers remove --app "APP_ID" --email "tester@example.com"`

## What to Test notes

- `asc builds test-notes create --build "BUILD_ID" --locale "en-US" --whats-new "Test instructions"`
- `asc builds test-notes update --id "LOCALIZATION_ID" --whats-new "Updated notes"`

## Critical rules

- NEVER skip the build/archive/export steps when doing a full publish. The IPA must be built fresh.
- NEVER assume a build is already on TestFlight without checking.
- Default to **external testing** for beta testers. Only use internal testing when the user explicitly asks or the tester is a known ASC team member.
- ALWAYS ask the user for confirmation before submitting for Beta App Review (`asc testflight review submit`).
- Do NOT call `beta-testers invite` for external testers — it fails before review approval. Testers are auto-invited once approved.
- Do NOT use `asc builds add-groups` for internal groups — they auto-receive builds.
- If any step fails, diagnose the error, fix it, and retry that step.
- Use `--paginate` on large groups/tester lists.
- Prefer IDs for deterministic operations.
- Always use `--help` to verify flags for the exact command.
