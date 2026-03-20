---
name: "user-assets"
description: "Handle user-pasted images: install as app icons, add to asset catalogs as named image sets, or use as design references. Use when the user attaches images."
tags: "assets, images, icons, pasted, clipboard"
---
# User-Pasted Image Integration

When the user pastes images, determine what to do with each one and integrate it into the project.

## Step 1: Determine Intent

Read the user's message and classify each image:

| Intent | Signals | Action |
|--------|---------|--------|
| **Design reference** | "make it look like this", "match this style", "here's a mockup" | Analyze visually to guide your code. Do NOT copy into the project. |
| **App icon** | "use this as the icon", "app icon", "this is the logo" | Copy to `AppIcon.appiconset/`, resize to platform size. See references/app-icon-installation. |
| **In-app image** | "add this image", "use this in the app", "background image", "splash" | Copy to `Assets.xcassets/` as a named image set. See references/image-asset-integration. |
| **Ambiguous** | No clear signal | Default: treat as an in-app image asset. |

## Step 2: Verify the Source Image

```bash
# Confirm file exists and get dimensions
sips -g pixelWidth -g pixelHeight /path/to/pasted/image.png
```

## Step 3: Integrate

Follow the appropriate reference:
- **App icon** → references/app-icon-installation
- **In-app image** → references/image-asset-integration

## Key Rules

1. **Always copy, never reference temp paths** — pasted images live in temp directories that are cleaned up. Use `cp` to copy into the project.
2. **Use `cp`, not `mv`** — preserve the original in case the session needs it again.
3. **Use `sips` for all image operations** — always available on macOS. No ImageMagick needed.
4. **Name files from context** — use meaningful names like `logo.png`, `hero-background.png`, not the temp filename.
5. **Reference in code correctly** — `Image("name")` for asset catalog images (no extension). `Bundle.main.url(forResource:)` for raw bundle resources.

## Cross-References

- **Asset catalog structure and Contents.json** → `asset-management` skill
- **Image containment/clip in SwiftUI** → `swiftui` skill
