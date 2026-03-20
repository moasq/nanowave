---
name: "image-asset-integration"
description: "Step-by-step process for adding a user-pasted image to the Xcode asset catalog as a named image set."
---
# Image Asset Integration

Use this when the user wants an image embedded in the app (logo, background, splash, custom graphic).

## 1. Choose a Name

Derive from context. Examples:
- "add this as the background" → `hero-background`
- "use this logo" → `logo`
- "splash screen image" → `splash`
- Generic → `custom-image`

## 2. Create the Image Set Directory

```bash
mkdir -p <AppName>/Assets.xcassets/<ImageName>.imageset
```

## 3. Copy the Image

```bash
cp /path/to/pasted/image.png <AppName>/Assets.xcassets/<ImageName>.imageset/<ImageName>.png
```

If format conversion is needed:
```bash
sips --setProperty format png /path/to/pasted/image.jpg --out <AppName>/Assets.xcassets/<ImageName>.imageset/<ImageName>.png
```

## 4. Write Contents.json

```json
{
  "images": [
    {
      "filename": "<ImageName>.png",
      "idiom": "universal"
    }
  ],
  "info": { "version": 1, "author": "xcode" }
}
```

Write this to `<AppName>/Assets.xcassets/<ImageName>.imageset/Contents.json`.

## 5. Reference in Code

```swift
// In SwiftUI — use the image set name (no extension, no path)
Image("<ImageName>")
    .resizable()
    .aspectRatio(contentMode: .fill)
```

## 6. Optional: Resize for Performance

If the source image is very large (e.g., 4000x3000) and will only display at a smaller size:

```bash
# Resize to reasonable dimensions for mobile
sips -Z 1200 /path/to/pasted/image.png --out <AppName>/Assets.xcassets/<ImageName>.imageset/<ImageName>.png
```

`-Z` resizes the longest edge while preserving aspect ratio.

## Multiple Images

If the user pastes multiple images for the same purpose (e.g., "add these to the gallery"):

```bash
# Create separate image sets for each
mkdir -p <AppName>/Assets.xcassets/gallery-1.imageset
mkdir -p <AppName>/Assets.xcassets/gallery-2.imageset
# ... copy each with its own Contents.json
```

Then reference them in code as an array:
```swift
let galleryImages = ["gallery-1", "gallery-2", "gallery-3"]
```
