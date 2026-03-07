---
name: asc-metadata-sync
description: Sync, validate, and translate App Store metadata and localizations with asc CLI, including LLM-powered translation and legacy metadata format migration. Use when updating metadata, translations, or adding new languages.
---

# asc metadata sync and localization

Use this skill to keep local metadata in sync with App Store Connect, and to translate metadata to multiple languages.

## Command discovery

- Always confirm flags with `--help` for the exact `asc` version.
- Prefer explicit long flags (`--app`, `--version`, `--version-id`, `--type`, `--app-info`).
- Default output is JSON; use `--output table` only for human verification steps.
- Prefer deterministic ID-based operations.

## Two Types of Localizations

### 1. Version Localizations (per-release)
Fields: `description`, `keywords`, `whatsNew`, `supportUrl`, `marketingUrl`, `promotionalText`

```bash
# List version localizations
asc localizations list --version "VERSION_ID"

# Download
asc localizations download --version "VERSION_ID" --path "./localizations"

# Upload from .strings files
asc localizations upload --version "VERSION_ID" --path "./localizations"
```

### 2. App Info Localizations (app-level)
Fields: `name`, `subtitle`, `privacyPolicyUrl`, `privacyChoicesUrl`, `privacyPolicyText`

```bash
# First, find the app info ID
asc app-infos list --app "APP_ID"

# List app info localizations
asc localizations list --app "APP_ID" --type app-info --app-info "APP_INFO_ID"

# Upload app info localizations
asc localizations upload --app "APP_ID" --type app-info --app-info "APP_INFO_ID" --path "./app-info-localizations"
```

**Note:** If you get "multiple app infos found", you must specify `--app-info` with the correct ID.

## Legacy Metadata Format Workflow

### Export current state
```bash
asc migrate export --app "APP_ID" --output "./metadata"
```

### Validate local files
```bash
asc migrate validate --help
```

### Import updates
```bash
asc migrate import --help
```

## Quick Field Updates

### Version-specific fields
```bash
asc app-info set --app "APP_ID" --locale "en-US" --whats-new "Bug fixes"
asc app-info set --app "APP_ID" --locale "en-US" --description "Your description"
asc app-info set --app "APP_ID" --locale "en-US" --keywords "keyword1,keyword2"
asc app-info set --app "APP_ID" --locale "en-US" --support-url "https://support.example.com"
```

### Version metadata
```bash
asc versions update --version-id "VERSION_ID" --copyright "2026 Your Company"
asc versions update --version-id "VERSION_ID" --release-type AFTER_APPROVAL
```

### TestFlight notes
```bash
asc build-localizations create --build "BUILD_ID" --locale "en-US" --whats-new "TestFlight notes"
```

## .strings File Format

Version localizations:
```
"description" = "Your app description";
"keywords" = "keyword1,keyword2,keyword3";
"whatsNew" = "What's new in this version";
"supportUrl" = "https://support.example.com";
```

App-info localizations:
```
"name" = "Your App Name";
"subtitle" = "Your subtitle";
"privacyPolicyUrl" = "https://example.com/privacy";
```

## Translation Workflow (LLM-powered)

### Step 1: Resolve IDs
```bash
asc apps list --output table
asc versions list --app "APP_ID" --state PREPARE_FOR_SUBMISSION --output table
asc app-infos list --app "APP_ID" --output table
```

### Step 2: Download source locale
```bash
asc localizations download --version "VERSION_ID" --path "./localizations"
asc localizations download --app "APP_ID" --type app-info --app-info "APP_INFO_ID" --path "./app-info-localizations"
```

### Step 3: Translate with LLM
See `references/translation-guidelines.md` for detailed translation rules, prompt template, and locale list.

### Step 4: Upload translations
```bash
# Version localizations
asc localizations upload --version "VERSION_ID" --path "./localizations"

# App-info localizations
asc localizations upload --app "APP_ID" --type app-info --app-info "APP_INFO_ID" --path "./app-info-localizations"
```

### Step 5: Verify
```bash
asc localizations list --version "VERSION_ID" --output table
asc localizations list --app "APP_ID" --type app-info --app-info "APP_INFO_ID" --output table
```

## Character Limits

| Field | Limit |
|-------|-------|
| Name | 30 |
| Subtitle | 30 |
| Keywords | 100 (comma-separated) |
| Description | 4000 |
| What's New | 4000 |
| Promotional Text | 170 |

**Always validate** translated text fits within limits before uploading. Use `asc migrate validate` to check.

## Agent Behavior

1. Always read the source locale first — never translate from memory.
2. Check existing localizations before overwriting.
3. Version vs app-info is different — use the right `--type` flag.
4. Prefer deterministic IDs — don't auto-pick via `head -1`.
5. Validate character limits before uploading.
6. Keywords: do NOT literally translate — research locale-appropriate search terms.
7. Show translations to the user before uploading for approval.
8. Process one locale at a time for easier review.
9. If upload fails for a locale, continue with others and report all failures at the end.

## Notes
- Version localizations are tied to a specific version.
- `promotionalText` can be updated anytime without a new version submission.
- `whatsNew` is only relevant for updates, not the first version.
- Privacy Policy URL is in app info localizations, not version localizations.
- For subscription/IAP display name localization, use `asc-subscription-localization` skill.
