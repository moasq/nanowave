---
name: asc-manual-actions
description: "Guide users through App Store Connect actions that require manual browser interaction. Use when agreements, tax/banking, privacy policies, or other dashboard-only tasks need attention."
---

# ASC Manual Actions

Some App Store Connect tasks cannot be completed via CLI or API. When these come up, guide the user through the required browser action with the exact URL and clear instructions.

## Agreements

Developer program agreements must be accepted in the browser.

When agreements are not active (pre-flight reports `AgreementsOK: false`):

```
Your developer agreements need attention before submission can proceed.

[OPTIONS]
- Open agreements page | Opens https://appstoreconnect.apple.com/agreements
- I've already accepted | Re-check agreement status
- Skip for now | Continue (submission may fail)
[/OPTIONS]
```

After the user says they've accepted, verify by re-running:
```bash
asc agreements list --output json
```

### Common agreement types
- **Paid Applications** — required for paid apps and in-app purchases
- **Free Applications** — required for all free apps
- **Apple Developer Program License** — the base agreement

## Tax and Banking

Required before receiving payments for paid apps or in-app purchases. Cannot be set via API.

```
Tax and banking information must be configured before paid app submission.

[OPTIONS]
- Open tax/banking page | Opens https://appstoreconnect.apple.com/agreements
- Already configured | Continue with submission
- Not applicable | This is a free app with no IAP
[/OPTIONS]
```

## Privacy Policy URL

Apple requires a privacy policy URL for apps that:
- Collect user data
- Have user accounts or sign-in
- Use third-party analytics or advertising
- Access device sensors (camera, location, contacts, etc.)

The AI **cannot generate a hosted URL** — the user must provide one they control.

```
A Privacy Policy URL is required. Do you have one?

[OPTIONS]
- Enter URL | [INPUT] Enter your privacy policy URL
- Generate policy | Create a privacy policy document I can host
- Not required | My app does not collect any user data
[/OPTIONS]
```

If the user selects "Generate policy":
- Generate a basic privacy policy markdown document
- Save it to the project directory as `PRIVACY_POLICY.md`
- Tell the user they need to host it (GitHub Pages, personal site, etc.) and provide the URL back
- Ask for the URL once they've hosted it

## Support URL

Apple requires a support URL for all apps. The AI **cannot create a hosted page**.

```
A Support URL is required. Do you have one?

[OPTIONS]
- Enter URL | [INPUT] Enter your support page URL
- Use GitHub profile | [INPUT] Enter your GitHub username
- Use email link | [INPUT] Enter your email address
[/OPTIONS]
```

If the user selects "Use GitHub profile", ask for their GitHub username and construct the URL.

## Content Rights Declaration

Required for all App Store submissions. Can be set via API but needs user confirmation:

```
Does your app use third-party content (images, audio, video, or text not created by you)?

[OPTIONS]
- No third-party content | My app only uses original content
- Uses third-party content | My app includes licensed or third-party content
[/OPTIONS]
```

Then set via:
```bash
asc apps update --id "APP_ID" --content-rights "DOES_NOT_USE_THIRD_PARTY_CONTENT"
# or
asc apps update --id "APP_ID" --content-rights "USES_THIRD_PARTY_CONTENT"
```

## Export Compliance (Encryption)

Required when a build uses non-exempt encryption:

```
Does your app use encryption beyond standard HTTPS/TLS?

[OPTIONS]
- No (standard HTTPS only) | Most apps — just uses HTTPS for network calls
- Yes, proprietary encryption | App implements custom encryption algorithms
- Yes, third-party encryption | App uses third-party encryption libraries
[/OPTIONS]
```

Better approach: Add `ITSAppUsesNonExemptEncryption = NO` to Info.plist and rebuild, avoiding this question entirely on future submissions.

## App Privacy Nutrition Label (Dashboard Only)

The privacy nutrition label **cannot be set via CLI** — it must be completed in the App Store Connect dashboard.

Check pre-flight context for `Data collection: detected` or `not detected`.

**If data collection is NOT detected:**

Tell the user:
> Your app doesn't appear to collect user data. Go to App Store Connect > your app > App Privacy > Get Started, and select **"No, we do not collect data from this app"**, then click Publish.

**If data collection IS detected:**

The pre-flight scan found data collection patterns. Guide the user:

```
Data collection patterns were detected in your project. The App Privacy nutrition label must declare what data your app collects.

[OPTIONS]
- Already configured | Privacy labels are set up in the dashboard
- Open dashboard | Open App Store Connect to configure privacy labels
- Help me identify | Help figure out what data my app collects
[/OPTIONS]
```

If the user selects "Help me identify", use the detected patterns to provide a specific checklist of data types to declare. See `asc-submission-prereqs` skill for the complete data types reference.

URL: `https://appstoreconnect.apple.com/apps` (select app > App Privacy)

## Third-Party AI Data Disclosure (Guideline 5.1.2(i))

**Effective November 2025.** If the app sends ANY personal data to a third-party AI (OpenAI, Anthropic, Google, etc.):

1. **Consent modal required in-app** — before first data transmission
2. **Must name the specific AI provider** (generic language is insufficient)
3. **Must describe what data types are sent** (text, images, voice)
4. **Must provide opt-out** for AI features involving data sharing
5. **Privacy policy must disclose** AI data sharing
6. **Privacy nutrition label must declare** relevant data types

This is a **hard rejection reason** — apps sharing data with AI without disclosure will be rejected.

```
Does your app send user data to any third-party AI service?

[OPTIONS]
- Yes, it uses AI APIs | App sends data to external AI services
- No AI services | App does not use external AI
- On-device AI only | All AI processing happens on device
[/OPTIONS]
```

If yes, verify the app code includes a consent mechanism before the first API call to the AI provider.

## App Review Information

Required for App Store submission. **Can be set via CLI.**

Fields:
- **Contact Info**: First name, last name, phone, email (for Apple to reach you during review)
- **Demo Account**: Username and password for a test account (only if app requires sign-in)
- **Notes**: Any special instructions for reviewers
- **Attachment**: Optional file (e.g., for hardware-dependent features)

When sign-in is detected in the project:

```
Your app has sign-in functionality. Apple reviewers need demo account credentials to test your app.

[OPTIONS]
- Enter credentials | [INPUT] Enter demo username and password (format: username / password)
- Not ready yet | I need to create a test account first
- Skip for now | Continue without (submission may be rejected)
[/OPTIONS]
```

After receiving credentials, set them via CLI:
```bash
# Check if review details already exist for this version
asc review details-for-version --version-id "VERSION_ID" --output json

# Create review details with demo credentials
asc review details-create --version-id "VERSION_ID" \
  --demo-account-required \
  --demo-account-name "USERNAME" \
  --demo-account-password "PASSWORD" \
  --contact-email "EMAIL" \
  --contact-first-name "FIRST" \
  --contact-last-name "LAST" \
  --contact-phone "PHONE"

# Or update existing review details
asc review details-update --id "DETAIL_ID" \
  --demo-account-required \
  --demo-account-name "USERNAME" \
  --demo-account-password "PASSWORD"
```

When no sign-in is detected:

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

## App Store Connect Dashboard URLs

Quick reference for manual actions:

| Action | URL |
|---|---|
| Agreements | https://appstoreconnect.apple.com/agreements |
| Tax/Banking | https://appstoreconnect.apple.com/agreements |
| API Keys | https://appstoreconnect.apple.com/access/integrations/api |
| Certificates | https://developer.apple.com/account/resources/certificates |
| App Privacy | https://appstoreconnect.apple.com/apps (select app > App Privacy) |
| App Review Info | https://appstoreconnect.apple.com/apps (select app > version > App Review) |
| Pricing | https://appstoreconnect.apple.com/apps (select app > Pricing and Availability) |

## Auto Mode Behavior

When the user selects "AI decides all", these manual-action items are **exceptions** — you MUST still ask about them because AI cannot resolve them autonomously:

1. **URLs the user must own** — Support URL, Privacy Policy URL
2. **Screenshots** — need actual images from the user or simulator
3. **Browser-only actions** — agreements, tax/banking (if flagged as incomplete)
4. **Content declarations** — content rights, encryption (legal liability on the user)

For everything else (description, keywords, age rating, copyright, what's new, category, promotional text), auto-fill without asking.
