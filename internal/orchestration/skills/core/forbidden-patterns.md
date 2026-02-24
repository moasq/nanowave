---
description: "Forbidden patterns - no networking, no third-party packages, no type re-declarations, no hardcoded styling; UIKit only when required"
---
# Forbidden Patterns

## Networking — BANNED
- No `URLSession`, no `Alamofire`, no REST clients
- No API calls of any kind
- No `async let` URL fetches
- The app is fully on-device

## UIKit — AVOID BY DEFAULT
- Prefer SwiftUI-first architecture
- `UIKit` imports are allowed only when a required feature has no viable SwiftUI equivalent
- `UIViewRepresentable` / `UIViewControllerRepresentable` are allowed only as minimal bridges for those required UIKit features
- No Storyboards, no XIBs, no Interface Builder

## Third-Party Packages — BANNED
- No SPM (Swift Package Manager) dependencies
- No CocoaPods
- No Carthage
- Use only Apple-native frameworks

## CoreData — BANNED
- Use **SwiftData** instead of CoreData
- No `NSManagedObject`, no `NSPersistentContainer`
- No `.xcdatamodeld` files

## Authentication & Cloud — BANNED
- No authentication screens, login flows, or token management
- No CloudKit, no iCloud sync
- No push notifications
- No Firebase, no Supabase, no backend services

## Hardcoded Styling — BANNED
- **NEVER** use hardcoded colors in views: `.white`, `.black`, `Color.red`, `Color.blue`, `.orange`, or `.opacity()` on raw colors
- **NEVER** use hardcoded fonts in views: `.font(.system(size:))`, `.font(.title2)`, `.font(.headline)`, etc.
- **NEVER** use hardcoded spacing: `.padding(20)`, `VStack(spacing: 10)`, etc.
- **ALL** colors must come from `AppTheme.Colors.*` tokens
- **ALL** fonts must come from `AppTheme.Fonts.*` tokens
- **ALL** spacing must come from `AppTheme.Spacing.*` tokens
- If a needed token doesn't exist in AppTheme, **add it to AppTheme first**, then reference it

```swift
// BANNED — hardcoded styling
.foregroundStyle(.white)
.font(.title2)
.font(.system(size: 48))
.padding(20)

// CORRECT — AppTheme tokens
.foregroundStyle(AppTheme.Colors.textPrimary)
.font(AppTheme.Fonts.title2)
.font(AppTheme.Fonts.largeTitle)
.padding(AppTheme.Spacing.lg)
```

## Type Re-declarations — BANNED
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
