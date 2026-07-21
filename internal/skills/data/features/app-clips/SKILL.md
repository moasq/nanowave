---
name: "app-clips"
description: "App Clip implementation with separate lightweight target, associated domains, NSUserActivity URL handling, SKOverlay full-app promotion, and size/API constraints. Use when adding an App Clip experience to an iOS app."
tags: "swiftui, ios, extensions"
platforms: "ios"
---
# App Clips

## Overview

App Clips are lightweight versions of an app for quick, focused tasks. Users launch them via NFC tags, QR codes, Maps, Safari banners, or Messages links — no App Store install required.

## Setup

Requires a separate App Clip target configured as `kind: "app_clip"` in the plan extensions array.

### Automatic Configuration

- **Info.plist**: `NSAppClip` dict with `NSAppClipRequestEphemeralUserNotification` and `NSAppClipRequestLocationConfirmation` is set automatically on the App Clip target in `project.yml`.
- **Associated Domains**: `appclips:{bundleID}` and `parent-application-identifiers` are configured automatically in `project.yml` entitlements.
- **Experience URL**: Configure in App Store Connect to define the invocation URL.

## URL Handling

Receive the invocation URL via `NSUserActivity`:

```swift
struct AppClipApp: App {
    var body: some Scene {
        WindowGroup {
            ContentView()
                .onContinueUserActivity(NSUserActivityTypeBrowsingWeb) { activity in
                    guard let url = activity.webpageURL else { return }
                    // Extract parameters from URL to show relevant content
                }
        }
    }
}
```

## Full App Promotion

Use `SKOverlay` to prompt users to download the full app:

```swift
import StoreKit

@Environment(\.requestAppStoreOverlay) var requestOverlay

Button("Get Full App") {
    requestOverlay(AppStoreOverlay.AppClipCompletion(appIdentifier: "YOUR_APP_ID"))
}
```

## Constraints

- Binary size must be under **15 MB**
- No access to HealthKit, CallKit, or SiriKit (use App Intents in the full app instead)
- Limited background execution modes
- Use `@AppStorage` for lightweight persistence — SwiftData is not available
