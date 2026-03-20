---
name: "app-icon-installation"
description: "Step-by-step process for installing a user-pasted image as the app icon."
---
# App Icon Installation

## 1. Find the Asset Catalog

```bash
find . -name "AppIcon.appiconset" -not -path "*/DerivedData/*" -not -path "*/.build/*"
```

Typical location: `<AppName>/Assets.xcassets/AppIcon.appiconset/`

## 2. Check Source Dimensions

```bash
sips -g pixelWidth -g pixelHeight /path/to/pasted/image.png
```

Minimum: **1024x1024** (tvOS: 1280x768).

## 3. Resize and Convert

```bash
# Resize to 1024x1024 + ensure PNG format
sips -z 1024 1024 --setProperty format png /path/to/pasted/image.png --out /tmp/AppIcon.png

# For tvOS (rectangular)
sips -z 768 1280 --setProperty format png /path/to/pasted/image.png --out /tmp/AppIcon.png
```

If the source is already 1024x1024 PNG, skip resize — just copy directly.

## 4. Copy into Asset Catalog

```bash
cp /tmp/AppIcon.png <AppName>/Assets.xcassets/AppIcon.appiconset/AppIcon.png
```

## 5. Write Contents.json

**iOS / visionOS:**
```json
{
  "images": [
    {
      "filename": "AppIcon.png",
      "idiom": "universal",
      "platform": "ios",
      "size": "1024x1024"
    }
  ],
  "info": { "version": 1, "author": "xcode" }
}
```

**macOS** — use `"idiom": "mac"`, `"scale": "1x"`, no `"platform"` key.

**watchOS** — use `"platform": "watchos"`.

**tvOS** — use `"idiom": "tv"`, `"platform": "tvos"`, `"size": "1280x768"`.

## 6. Verify

```bash
sips -g pixelWidth -g pixelHeight <AppName>/Assets.xcassets/AppIcon.appiconset/AppIcon.png
cat <AppName>/Assets.xcassets/AppIcon.appiconset/Contents.json
```

Then run `nw_xcode_build` to confirm Xcode accepts it.
