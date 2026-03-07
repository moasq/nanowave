---
name: asc-version-routing
description: Route App Store Connect actions based on version state. Use when the user wants to submit, update, or manage an app version and you need to determine the correct workflow based on the current version state.
---

# Version State Routing

Before doing ANY metadata or submission work, check the pre-flight version state from the system prompt and route accordingly.

## Version State Reference

| State | Editable? | Description |
|-------|-----------|-------------|
| `PREPARE_FOR_SUBMISSION` | Yes | Version created, ready for metadata and submission |
| `DEVELOPER_REJECTED` | Yes | Developer pulled it back from review, editable again |
| `WAITING_FOR_REVIEW` | No | Submitted, waiting in Apple's queue |
| `IN_REVIEW` | No | Apple is actively reviewing |
| `REJECTED` | No | Apple rejected the submission |
| `READY_FOR_SALE` | No | Live on the App Store |
| `PENDING_DEVELOPER_RELEASE` | No | Approved, waiting for developer to release |
| `PROCESSING_FOR_APP_STORE` | No | Apple is processing after approval |

## First Action: Route by State

| State | Action |
|-------|--------|
| `PREPARE_FOR_SUBMISSION` | Proceed with metadata and submission normally |
| `DEVELOPER_REJECTED` | Inform user this is a resubmission, then proceed normally |
| `WAITING_FOR_REVIEW` | Tell user the version is in queue. Ask: cancel review, create new version, or wait |
| `IN_REVIEW` | Tell user Apple is reviewing. Ask: cancel review (warn loses queue slot), create new version, or wait |
| `REJECTED` | Ask: view rejection reasons, or create a new version |
| `READY_FOR_SALE` | This is the live version. Ask what version number for the update |
| `PENDING_DEVELOPER_RELEASE` | Ask: release now, create new version, or wait |
| `PROCESSING_FOR_APP_STORE` | Tell user to wait and re-check later, nothing can be done now |
| No versions exist | Ask what version number for the first release |

## Creating a New Version

When a new version is needed (live app update, after rejection, etc.):

1. Ask user for version number with a suggestion based on the current live version.
2. Create via: `asc versions create --app APP_ID --version VERSION --platform IOS`
3. If building is also needed, the `MARKETING_VERSION` in the Xcode project must match the new version number. Update it before building.

## Cancelling a Submission

When user wants to cancel `WAITING_FOR_REVIEW` or `IN_REVIEW`:

```bash
asc submit status --app APP_ID --output json  # find submission ID
asc submit cancel --id SUBMISSION_ID --confirm
```

After cancellation, version returns to `PREPARE_FOR_SUBMISSION` and can proceed normally.

## Version Number + Build Number

- **Version number** (`MARKETING_VERSION` / `CFBundleShortVersionString`): user-facing, e.g. "1.1" - must match the ASC version.
- **Build number** (`CURRENT_PROJECT_VERSION` / `CFBundleVersion`): internal, must be unique and higher than any previous upload.
- When creating a new version, check if the Xcode project's `MARKETING_VERSION` matches and update it if not.

## Multiple Versions

When multiple non-live versions exist (e.g. 1.0 `WAITING_FOR_REVIEW` + 1.1 `PREPARE_FOR_SUBMISSION`):

- Report all of them to the user.
- Ask which one to work with.
- If user's intent matches an existing editable version, use it.
