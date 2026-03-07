---
name: asc-screenshot-upload
description: "Handle App Store screenshot requirements including automatic simulator capture and validated browser upload. Use when screenshots are missing or need to be uploaded for App Store submission."
---

# ASC Screenshot Upload

## Two-Option Screenshot System

During pre-flight step 8, the user chooses how to provide screenshots:

### Automatic (Recommended)

Pre-flight builds the app for each required simulator, boots them, installs, launches, and captures the initial launch screen. Simulators are left running so you can capture additional screens.

After pre-flight, use AXe to navigate to key screens and capture more:

```bash
# See current screen elements
axe describe-ui --udid <UDID>

# Navigate to a screen
axe tap --id <element_id> --udid <UDID>

# Capture screenshot
xcrun simctl io <UDID> screenshot <path>.png
```

Analyze the app's Swift source to identify important screens (ignore settings/preferences). Capture 3-5 screenshots showing the app's core functionality.

### Custom Upload

Browser UI with Apple validation:

- **Requirements checklist** at top shows which device types are needed (green checkmarks when fulfilled)
- Each uploaded image shows a **badge**: green with device type if dimensions match, red "Invalid size" if not
- **Upload Screenshots** button disabled until all required types have valid screenshots
- **Upload Anyway** escape hatch for incomplete uploads
- Summary line: "2/2 required types covered" or "1/2 — iPad 13" still needed"

## Device Family Requirements

The `device_family` field in `project_config.json` determines what's required:

| Device Family | Required Screenshots |
|---|---|
| `iphone` (default) | iPhone 6.9" (or 6.5" fallback) |
| `ipad` | iPad 13" |
| `universal` | iPhone 6.9" AND iPad 13" |

**iPad screenshots are mandatory for universal/iPad apps.** Submission will fail without them.

## Apple Screenshot Dimensions

| Device | Resolution (portrait) | Device Type |
|---|---|---|
| iPhone 6.9" | 1320x2868, 1290x2796, 1260x2736 | IPHONE_69 |
| iPhone 6.5" (fallback) | 1284x2778, 1242x2688 | IPHONE_65 |
| iPad 13" | 2064x2752, 2048x2732 | IPAD_PRO_13 |

Format: PNG or JPEG, RGB, no transparency. 1-10 screenshots per device type per locale.

## Screenshot Directory Convention

```
screenshots/
  auto/       # Automatic simulator captures
  upload/     # User-provided screenshots from browser upload
  framed/     # Framed screenshots ready for upload
```

## Uploading to App Store Connect

```bash
# Get the version localization ID
asc localizations list --version "VERSION_ID" --output json

# Upload screenshots
asc screenshots upload \
  --version-localization "LOC_ID" \
  --path "./screenshots/auto" \
  --device-type "IPHONE_69" \
  --output json
```

## Screenshot Status in System Prompt

The system prompt reports screenshot status:
- `available at <path> (device types: IPHONE_69, IPAD_PRO_13)` — ready
- `available at <path> (not yet validated)` — found but unchecked
- `NOT available` — missing, required before submission

If automatic capture was used, simulator UDIDs are provided for multi-screen capture via AXe.
