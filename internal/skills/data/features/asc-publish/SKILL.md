---
name: "asc-publish"
description: "App Store publishing workflow with readiness checks, metadata preview, and submission. Use when publishing to the App Store, submitting to TestFlight, or managing App Store metadata."
---
# App Store Publishing

## Important: iOS 26 SDK Requirement

Starting April 28, 2026, all apps submitted to the App Store must be built with the iOS 26 SDK.
Ensure the project targets iOS 26 SDK before submission.

## Publishing Flow

The automated pipeline handles both TestFlight and App Store targets based on user intent.
When no Xcode GUI is available, use `xcodebuild` commands directly for archive and export operations.

## Publishing Workflow (MANDATORY)

For ANY operation that writes to App Store Connect, follow these steps in order.
For read-only operations (list_apps, get_app_status, list_builds, check_auth, list_certificates, list_profiles, list_bundle_ids, validate_version), proceed directly.

### Step 1: GATHER

Call read-only tools to check current state:
- `get_app_status` for release pipeline state
- `validate_version` for what's complete vs missing
- `list_builds` for available builds and processing status

### Step 2: READINESS REPORT

Show a checklist using these markers:

```
  [checkmark] App registered in ASC
  [checkmark] Bundle ID: com.example.app
  [checkmark] Build 1.0 (42) -- processed, ready
  [x] Screenshots -- missing for iPhone 6.7"
  [x] Description -- not set
  [checkmark] Age rating -- set
```

For items the user must fix outside ASC tools, provide the exact URL:
- Agreements: https://appstoreconnect.apple.com/agreements
- Tax/Banking: https://appstoreconnect.apple.com/agreements
- Certificates: https://developer.apple.com/account/resources/certificates
- Privacy policy: must be hosted by the user (provide guidance)

### Step 3: PREVIEW ALL FIELDS

Show a formatted preview of everything that will be pushed:

```
APP STORE SUBMISSION PREVIEW
App Name:      WeatherApp
Bundle ID:     com.janedoe.weatherapp
Version:       1.0
Build:         42
Platform:      iOS

Description:   [AI-generated]
  "A beautiful weather app..."

Keywords:      [AI-generated]
  weather, forecast, rain, temperature

What's New:    [AI-generated]
  "Initial release"

Age Rating:    4+
Localizations: English (primary)
```

Mark ALL AI-generated values with [AI-generated].
Ask: "Does everything look correct? Type 'yes' to proceed, or tell me what to change."

### Step 4: APPLY CORRECTIONS

If the user wants changes, update the values and re-show the full preview.
Repeat until confirmed.

### Step 5: PUSH AND SUBMIT

Only after explicit "yes":
1. `set_metadata` -- push metadata
2. `set_age_rating` -- set rating
3. `validate_version` -- final readiness check
4. `submit_for_review` -- with one last confirmation

CRITICAL: Even if the user says "just do it" or "skip the preview", you MUST show the preview. It is the user's last chance to catch errors before submission.

## Mandatory User Actions

These cannot be automated and require user action:
- **Privacy Policy URL**: Must be hosted by the user. Required for apps with accounts or data collection.
- **Support URL**: Must be a valid URL the user controls.
- **Apple Developer Program agreements**: Must be signed in browser.
- **Tax and banking setup**: Must be completed in App Store Connect.

When any of these are missing, tell the user exactly what to do and provide the URL.

## Localization

When localizations are configured:
- Generate metadata for EACH language
- Show ALL translations in the preview
- Label each as [AI-generated]
- User can correct any language individually
