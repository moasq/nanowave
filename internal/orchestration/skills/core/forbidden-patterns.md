---
description: "Forbidden patterns - no networking, no unapproved third-party packages, no type re-declarations, no hardcoded styling; UIKit only when required"
---
# Forbidden Patterns

## Networking — BANNED
**Why:** Generated apps must work 100% offline.
- No `URLSession`, no `Alamofire`, no REST clients
- No API calls of any kind
- No `async let` URL fetches
- The app is fully on-device

## UIKit — AVOID BY DEFAULT
**Why:** SwiftUI-first ensures cross-platform consistency.
- Prefer SwiftUI-first architecture
- `UIKit` imports are allowed only when a required feature has no viable SwiftUI equivalent
- `UIViewRepresentable` / `UIViewControllerRepresentable` are allowed only as minimal bridges for those required UIKit features
- No Storyboards, no XIBs, no Interface Builder

## Third-Party Packages — SPM Only, Validated Before Use
**Why:** Dependencies add build complexity and supply chain risk. Use Apple frameworks first; add an SPM package only when it provides a meaningfully better experience.

Use SPM (Swift Package Manager) as the only package manager. CocoaPods and Carthage are not allowed.

### When to add an SPM package

Add an SPM package when any of these apply:
- A feature needs capabilities that no Apple framework provides (e.g. playing After Effects / Lottie JSON animations, advanced image processing pipelines, barcode generation beyond Core Image).
- The approved-packages list below explicitly names a package for this project.
- The user's prompt explicitly requests a specific package by name.

Use native Apple frameworks when they cover the need well enough. For example, use Swift Charts instead of a charting library, use `AsyncImage` or PhotosUI instead of an image-loading library, and use `Canvas` / `TimelineView` for simple particle animations.

### How to validate a package before adding it

Search the internet for the package. Then confirm each item:
1. The GitHub repository exists and has more than 500 stars.
2. The repository was updated within the last 12 months.
3. The license is MIT or Apache 2.0.
4. A `Package.swift` file exists in the repository root (confirms SPM support).
5. Read the README to learn the correct import name and SwiftUI usage.

After validation, use the `add_package` MCP tool to add it to the Xcode project. If the tool is unavailable, edit `project.yml` directly (see the SPM Packages section in the build plan for the exact YAML format).

### SPM naming — three names to keep straight

SPM has three different names for the same dependency. Getting them confused is the most common integration error.

| Name | What it is | Example for Lottie |
|---|---|---|
| **Repository name** | Last path component of the Git URL. Used as the XcodeGen `packages:` key and in `- package:` under dependencies. | `lottie-ios` |
| **Package display name** | The `name:` in `Package.swift`. Only for display — SPM resolves by URL, not by this name. | `Lottie` |
| **Product name** | Declared in `products: [.library(name: "…")]` inside `Package.swift`. This is what you write in `import` statements and in the `product:` field in XcodeGen. | `Lottie` |

To find the correct product name, open the repository's `Package.swift` on GitHub and look at the `products:` array.

<!-- APPROVED_PACKAGES_PLACEHOLDER -->

## CoreData — BANNED
**Why:** SwiftData is Apple's modern replacement.
- Use **SwiftData** instead of CoreData
- No `NSManagedObject`, no `NSPersistentContainer`
- No `.xcdatamodeld` files

## Authentication & Cloud — BANNED
**Why:** Beyond on-device MVP scope.
- No authentication screens, login flows, or token management
- No CloudKit, no iCloud sync
- No push notifications
- No Firebase, no Supabase, no backend services

## Hardcoded Styling — BANNED
**Why:** Centralized tokens ensure consistency, enable theme changes.
- **NEVER** use hardcoded colors in views: `.white`, `.black`, `Color.red`, `Color.blue`, `.orange`, or `.opacity()` on raw colors
- **NEVER** use hardcoded fonts in views: `.font(.system(size:))`, `.font(.title2)`, `.font(.headline)`, etc.
- **NEVER** use hardcoded spacing: `.padding(20)`, `VStack(spacing: 10)`, etc.
- **ALL** colors must come from `AppTheme.Colors.*` tokens
- **ALL** fonts must come from `AppTheme.Fonts.*` tokens
- **ALL** spacing must come from `AppTheme.Spacing.*` tokens
- Text color tokens MUST use UIKit adaptive colors (`Color(.label)`, `Color(.secondaryLabel)`, `Color(.tertiaryLabel)`) — these auto-adapt to appearance mode
- Brand/theme color tokens (primary, secondary, accent, background, surface) use `Color(hex:)` with palette values
- If a needed token doesn't exist in AppTheme, **add it to AppTheme first**, then reference it

```swift
// BANNED — hardcoded styling
.foregroundStyle(.white)
.foregroundStyle(.black)
.foregroundStyle(Color.primary)
.font(.title2)
.font(.system(size: 48))
.padding(20)

// CORRECT — AppTheme tokens (text uses UIKit adaptive, brand uses hex)
.foregroundStyle(AppTheme.Colors.textPrimary)   // Color(.label) — adapts to appearance
.foregroundStyle(AppTheme.Colors.accent)         // Color(hex:) — brand identity
.font(AppTheme.Fonts.title2)
.font(AppTheme.Fonts.largeTitle)
.padding(AppTheme.Spacing.lg)
```

## Type Re-declarations — BANNED
**Why:** Duplicates cause ambiguous type errors.
- **NEVER** re-declare types that exist in other project files
- **NEVER** re-declare types from SwiftUI/Foundation (`Color`, `CGPoint`, `Font`, etc.)
- Import the module or file that defines the type
- Each type must be defined in **exactly one file**

```swift
// BANNED — re-declaring Color enum that SwiftUI already provides
enum Color {
    case red, blue, green
}

// BANNED — re-declaring a model that exists in Models/Note.swift
struct Note {
    var title: String
}

// CORRECT — import and use the existing type
import SwiftUI  // provides Color
// Note is already defined in Models/Note.swift, just reference it
```
