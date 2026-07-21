---
name: "safari-extension"
description: "Safari Web Extension integration with content scripts, background service workers, native messaging via SFSafariExtensionHandler, and manifest v3 configuration. Use when adding a Safari browser extension target to an iOS or macOS app."
tags: "swiftui, ios, extensions"
platforms: "ios"
---
# Safari Web Extension

## Overview

Safari Web Extensions use web technologies (JS/HTML/CSS) plus a native Swift wrapper for browser integration. The extension runs as a separate target alongside the main app.

## Setup

Requires a separate extension target configured as `kind: "safari"` in the plan extensions array.

## Native Communication Handler

Implement `SFSafariExtensionHandler` for bidirectional messaging between Swift and JavaScript:

```swift
import SafariServices

class SafariExtensionHandler: SFSafariExtensionHandler {
    override func toolbarItemClicked(in window: SFSafariWindow) {
        window.getActiveTab { tab in
            tab?.getActivePage { page in
                page?.dispatchMessageToScript(withName: "buttonClicked", userInfo: [:])
            }
        }
    }

    override func messageReceived(withName messageName: String, from page: SFSafariPage, userInfo: [String: Any]?) {
        // Handle messages sent from JS via browser.runtime.sendNativeMessage()
    }
}
```

## Info.plist Configuration

Set these keys in the extension target's Info.plist (via XcodeGen):

```
NSExtensionPrincipalClass: $(PRODUCT_MODULE_NAME).SafariExtensionHandler
NSExtensionAttributes:
  SFSafariWebsiteAccess: { Level: All }
```

## Web Extension Resources

Place these files in the extension bundle:

- `manifest.json` — Web Extension manifest v3
- `background.js` — service worker for background logic
- `content.js` — injected into web pages for DOM access
- `popup.html` — toolbar popup UI (optional)

## User Activation

The user must manually enable the extension in Safari → Settings → Extensions before it becomes active.
