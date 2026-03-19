---
name: "asset-management"
description: "Manage Xcode project assets including app icons, images, and asset catalogs. Use when handling app icon installation, image asset creation, or asset catalog modifications."
---
# Asset Management

## Asset Catalog Structure

Xcode projects use `.xcassets` bundles containing named asset sets:

```
Assets.xcassets/
  Contents.json                    # Root catalog manifest
  AppIcon.appiconset/
    Contents.json                  # Icon set manifest with image references
    AppIcon.png                    # The actual icon file
  AccentColor.colorset/
    Contents.json                  # Color set manifest
```

## App Icon Requirements by Platform

All platforms require a single high-resolution icon. Xcode generates all needed sizes automatically.

| Platform | Size | Idiom | Platform Key |
|----------|------|-------|-------------|
| iOS | 1024x1024 | universal | ios |
| watchOS | 1024x1024 | universal | watchos |
| tvOS | 1280x768 | tv | tvos |
| macOS | 1024x1024 | mac | (none) |
| visionOS | 1024x1024 | universal | xros |

## Contents.json Format

iOS example:
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
  "info": {
    "version": 1,
    "author": "xcode"
  }
}
```

macOS uses `"idiom": "mac"` with `"scale": "1x"` and no `"platform"` key.

## Image Processing with sips

macOS includes `sips` (scriptable image processing system) for image manipulation:

```bash
# Query dimensions
sips -g pixelWidth -g pixelHeight image.png

# Resize to exact dimensions
sips -z 1024 1024 image.png

# Convert format to PNG
sips --setProperty format png image.jpg --out image.png

# Resize and convert in one step
sips -z 1024 1024 --setProperty format png input.jpg --out output.png
```

## Verifying Icon Installation

To verify an icon is correctly installed:

1. Check `AppIcon.appiconset/Contents.json` has a `filename` entry
2. Verify the referenced file exists in the same directory
3. Confirm dimensions match the platform requirement using `sips -g pixelWidth -g pixelHeight`
4. Build the project to verify Xcode accepts the icon

## Finding the Asset Catalog

Search for the asset catalog in the project:
```bash
find . -name "AppIcon.appiconset" -not -path "*/DerivedData/*" -not -path "*/.build/*"
```

Or use Glob: `**/AppIcon.appiconset/`
