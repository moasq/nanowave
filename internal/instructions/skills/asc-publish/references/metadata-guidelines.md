---
name: "asc-publish-metadata"
description: "App Store metadata character limits, requirements, and optimization tips. Use when generating or reviewing App Store metadata."
---
# Metadata Guidelines

## Field Limits

| Field | Max Length | Required | Notes |
|---|---|---|---|
| App Name | 30 chars | Yes | Shown on App Store. Cannot be changed often. |
| Subtitle | 30 chars | No | Short tagline below the name |
| Description | 4000 chars | Yes | Searchable. First 3 lines visible before "more". |
| Keywords | 100 chars | Yes | Comma-separated. Not visible to users. |
| What's New | 4000 chars | Yes (updates) | Release notes for this version |
| Support URL | URL | Yes | Must be a valid, accessible URL |
| Marketing URL | URL | No | Link to your marketing page |
| Privacy Policy URL | URL | Conditional | Required if app has accounts or collects data |
| Promotional Text | 170 chars | No | Can be updated without new version. Above description. |

## Keyword Optimization

- Use all 100 characters (comma-separated, no spaces after commas)
- Do not repeat words from the app name (they are already indexed)
- Use singular forms (Apple indexes both singular and plural)
- Avoid generic terms (free, app, best) -- they are too competitive
- Include common misspellings if relevant
- Separate compound words (note,book instead of notebook)

## Description Best Practices

- Lead with value proposition in first 3 lines (visible before "more")
- Use short paragraphs and line breaks for readability
- Include key features as a bulleted list
- End with a call to action
- Do not include prices (they change by region)
- Do not mention competing platforms

## Age Rating Categories

| Category | Description |
|---|---|
| 4+ | No objectionable content |
| 9+ | Mild or infrequent cartoon/fantasy violence |
| 12+ | Infrequent mild language, simulated gambling |
| 17+ | Frequent intense violence, mature themes |

Set via `set_age_rating`. Most utility and productivity apps are 4+.

## iOS 26 SDK Deadline

Starting April 28, 2026, all apps submitted to the App Store must be built with the iOS 26 SDK.
Apps built with older SDKs will be rejected during submission.

## Common Rejection Reasons to Avoid

- Missing privacy policy URL when app collects data
- Placeholder or lorem ipsum text in metadata
- Screenshots that do not match current app version
- Description claims features the app does not have
- Missing purpose string for permissions used (camera, location, etc.)
- Broken support URL or privacy policy URL
- App built with SDK older than iOS 26 (after April 28, 2026)
- Missing data collection declarations in App Privacy section
- In-app purchase items not configured or missing metadata
- App crashes on launch or during review (test on physical device before submitting)
