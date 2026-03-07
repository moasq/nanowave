---
name: asc-submission-prereqs
description: "Complete App Store submission prerequisites checklist including privacy, data collection, AI disclosure, screenshots, and metadata requirements. Use when preparing an app for App Store review or diagnosing submission failures."
---

# App Store Submission Prerequisites

Complete checklist of everything required before `asc submit` can succeed. Work through each section in order. Use pre-flight context to skip items already verified.

## 1. Build Requirements

- Build uploaded and in `VALID` processing state
- Built with iOS 26 SDK or later (required after April 28, 2026)
- Signing certificate and provisioning profile valid
- Build number unique (higher than any previous upload)

Check: `asc builds list --app APP_ID --sort -uploadedDate --limit 1 --output json`

## 2. Required Metadata (per locale)

All fields must be set for each localization:

| Field | Max | Required | Notes |
|---|---|---|---|
| Description | 4000 chars | Yes | First 3 lines visible before "more" |
| Keywords | 100 chars | Yes | Comma-separated, no spaces after commas |
| What's New | 4000 chars | Yes (updates) | Release notes |
| Support URL | URL | Yes | Must be live and accessible |
| Copyright | text | Yes | e.g. "2026 Company Name" |

Check: `asc localizations list --version VERSION_ID --output json`

## 3. Privacy Requirements (CRITICAL)

### 3a. Privacy Policy URL

**Required for ALL apps.** Must be:
- Publicly accessible URL (not behind login)
- Set in both App Store Connect metadata AND accessible within the app
- Must explicitly state: what data is collected, how, all uses, third-party recipients

Ask user:
```
A Privacy Policy URL is required for App Store submission. Do you have one?

[OPTIONS]
- Enter URL | [INPUT] Enter your privacy policy URL
- Help me create one | Generate a policy document I can host
- Not sure | Help me determine if I need one
[/OPTIONS]
```

### 3b. App Privacy Nutrition Label (Dashboard Only)

**Cannot be set via CLI** — must be completed in App Store Connect dashboard.

Check pre-flight context for `Data collection: detected` or `not detected`.

**If data collection is NOT detected:**

Tell the user:
> Your app doesn't appear to collect user data. Go to App Store Connect > your app > App Privacy > Get Started, and select **"No, we do not collect data from this app"**, then click Publish.

Navigation: `https://appstoreconnect.apple.com/apps` (select app > App Privacy)

**If data collection IS detected:**

The pre-flight scan found data collection patterns in the project code. Guide the user through the privacy nutrition label declaration:

1. Identify which data types are collected based on detected patterns
2. For each data type, declare the **purpose**: Third-Party Advertising, Developer's Advertising/Marketing, Analytics, Product Personalization, App Functionality, or Other
3. Direct the user to App Store Connect > App Privacy > Get Started

See `references/privacy-data-types.md` for the complete data types list.

Categories include: Contact Info, Health & Fitness, Financial Info, Location, Sensitive Info, Contacts, User Content, Browsing/Search History, Identifiers, Purchases, Usage Data, Diagnostics.

```
The App Privacy nutrition label must be configured in App Store Connect.

[OPTIONS]
- Already configured | I've set up privacy labels in the dashboard
- Open dashboard | Open App Store Connect to configure privacy labels
- Help me identify | Help me figure out what data my app collects
[/OPTIONS]
```

URL: `https://appstoreconnect.apple.com/apps` (select app > App Privacy)

### 3c. Third-Party AI Data Disclosure (Guideline 5.1.2(i))

**If the app sends ANY user data to a third-party AI service** (OpenAI, Anthropic, Google, etc.):

1. **In-app consent modal required** — must appear BEFORE first data transmission
2. **Must name the specific AI provider** (e.g. "OpenAI", "Anthropic Claude")
3. **Must describe what data is sent** (text, images, voice, etc.)
4. **Must allow user to opt out** of AI features that share data
5. **Privacy policy must disclose** the AI data sharing

Ask user:
```
Does your app send any user data to a third-party AI service (OpenAI, Claude, Gemini, etc.)?

[OPTIONS]
- Yes, it uses AI APIs | App sends data to external AI services
- No AI services | App does not communicate with external AI
- Uses on-device AI only | AI processing happens entirely on device
[/OPTIONS]
```

If yes, verify the app includes:
- Consent modal naming the provider and data types
- Opt-out mechanism for AI features
- Privacy policy disclosure

### 3d. Data Collection from LLM/Backend

Common patterns that require privacy disclosure:

| Pattern | Data Types to Declare |
|---|---|
| Text sent to LLM API | User Content (Other), Usage Data |
| Images sent to AI vision | Photos or Videos, User Content |
| Voice/audio to speech API | Audio Data, User Content |
| Analytics SDK (Firebase, etc.) | Usage Data, Diagnostics, Device ID |
| Crash reporting (Sentry, etc.) | Diagnostics (Crash Data, Performance) |
| User accounts/auth | Contact Info, Identifiers |
| Push notifications | Identifiers (Device ID) |
| Location features | Location (Precise or Coarse) |
| In-app purchases | Purchases, Financial Info |
| Backend API with user data | Depends on what's transmitted |

### 3e. App Review Information

**Can be set via CLI** using `asc review` commands.

Required fields:
- **Contact Info**: First name, last name, phone number, email
- **Sign-In Required**: If the app has sign-in, you MUST provide demo account credentials
- **Notes**: Optional reviewer notes (e.g. "No sign-in required" or test instructions)

Check pre-flight context for `Project sign-in: detected/not detected`.

**If sign-in detected:**

```
Your app has sign-in functionality. Apple requires demo account credentials for the review team.

[OPTIONS]
- Enter credentials | [INPUT] Enter demo username and password (format: username / password)
- Not ready yet | I need to create a test account first
- Skip for now | Continue without (submission may be rejected)
[/OPTIONS]
```

After receiving credentials, set them via CLI:
```bash
# First check if review details exist
asc review details-for-version --version-id "VERSION_ID" --output json

# Create or update review details with demo credentials
asc review details-create --version-id "VERSION_ID" \
  --demo-account-required \
  --demo-account-name "USERNAME" \
  --demo-account-password "PASSWORD" \
  --contact-email "EMAIL" \
  --contact-first-name "FIRST" \
  --contact-last-name "LAST" \
  --contact-phone "PHONE"

# Or update existing details
asc review details-update --id "DETAIL_ID" \
  --demo-account-required \
  --demo-account-name "USERNAME" \
  --demo-account-password "PASSWORD"
```

**If no sign-in detected:**

Set review details without demo credentials:
```bash
asc review details-create --version-id "VERSION_ID" \
  --contact-email "EMAIL" \
  --contact-first-name "FIRST" \
  --contact-last-name "LAST" \
  --contact-phone "PHONE" \
  --notes "No sign-in required"
```

Tell the user: "Your app doesn't require sign-in, so demo credentials aren't needed."

## 4. Age Rating

Must be set. Ask if not configured:
```
What age rating fits your app?

[OPTIONS]
- 4+ | No objectionable content
- 9+ | Mild cartoon or fantasy violence
- 12+ | Infrequent mild language, simulated gambling
- 17+ | Frequent intense violence, mature themes
[/OPTIONS]
```

Check: `asc apps get --id APP_ID --output json` (look for `contentRightsDeclaration`)

## 5. Content Rights Declaration

Required for all submissions:
```
Does your app use third-party content (images, audio, video, or text not created by you)?

[OPTIONS]
- No third-party content | Only original content
- Uses third-party content | Includes licensed or third-party material
[/OPTIONS]
```

Set: `asc apps update --id APP_ID --content-rights DOES_NOT_USE_THIRD_PARTY_CONTENT`

## 6. Export Compliance (Encryption)

Required if app uses encryption beyond standard HTTPS:
```
Does your app use encryption beyond standard HTTPS/TLS?

[OPTIONS]
- No, standard HTTPS only | Most apps — just uses HTTPS for networking
- Yes, custom encryption | App implements custom or third-party encryption
[/OPTIONS]
```

Best approach: Add `ITSAppUsesNonExemptEncryption = NO` to Info.plist before building.

## 7. Screenshots

Required per device type the app supports. Minimum 1, maximum 10 per locale.

**For iPhone apps, required sizes (most common):**
- 6.9" display: 1260x2736 (iPhone 17 Pro Max, iPhone Air)
- 6.5" display: 1284x2778 (fallback if 6.9" not provided)

See `references/screenshot-sizes.md` for all device sizes.

Check: `asc screenshots list --version-localization LOC_ID --output table`

If missing, use OPTIONS:
```
Screenshots are required. How would you like to provide them?

[OPTIONS]
- Upload screenshots | Open browser to drag-and-drop images
- Capture from simulator | Auto-capture from the running app
- Skip for now | Continue without (submission will fail)
[/OPTIONS]
```

## 8. App Icon

- 1024x1024 PNG required
- Pre-flight already checks this — see `IconReady` flag

## 9. Agreements and Tax/Banking

- Developer agreements must be ACTIVE (pre-flight checks this)
- Paid apps require tax and banking setup
- Both must be done in browser: https://appstoreconnect.apple.com/agreements

## 10. Category

Primary category required. Ask if not set:
```
What category best fits your app?

[OPTIONS]
- Utilities | Tools and utilities
- Productivity | Work and task management
- Finance | Banking, payments, budgeting
- Health & Fitness | Health tracking, workouts
- Education | Learning and teaching
- Entertainment | Fun and leisure
- Social Networking | Communication and social
- Games | Interactive games
- Other | I'll specify
[/OPTIONS]
```

## Pre-Submission Verification Order

1. Build status (VALID, not PROCESSING)
2. Metadata complete (description, keywords, what's new, copyright)
3. URLs provided (support URL, privacy policy URL)
4. Privacy nutrition label configured (dashboard)
5. AI data disclosure in place (if applicable)
6. Age rating set
7. Content rights declared
8. Export compliance resolved
9. Screenshots uploaded
10. App icon ready
11. Agreements active
12. Category set
13. App Review Information set (if sign-in required)
14. App Privacy nutrition label configured (dashboard)

Only after ALL items pass: show preview, then submit.
