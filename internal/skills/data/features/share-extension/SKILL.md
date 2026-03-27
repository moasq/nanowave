---
name: "share-extension"
description: "Share extension with NSExtensionActivationRule filtering, SLComposeServiceViewController handling, App Group data sharing, and shared content processing for URLs, text, and images. Use when adding share sheet integration to an iOS app."
tags: "swiftui, ios, extensions"
platforms: "ios"
---
# Share Extension

## Overview

The share extension receives content (URLs, text, images) from other apps via the system share sheet. It runs as a separate process with limited memory and no direct access to the main app's data unless App Groups are configured.

## Setup

Requires a separate extension target configured as `kind: "share"` in the plan extensions array.

## Principal View Controller

Implement `SLComposeServiceViewController` to handle shared content:

```swift
import Social
import UniformTypeIdentifiers

class ShareViewController: SLComposeServiceViewController {
    override func isContentValid() -> Bool {
        return contentText.count > 0
    }

    override func didSelectPost() {
        guard let item = extensionContext?.inputItems.first as? NSExtensionItem,
              let provider = item.attachments?.first else {
            extensionContext?.completeRequest(returningItems: [], completionHandler: nil)
            return
        }

        if provider.hasItemConformingToTypeIdentifier("public.url") {
            provider.loadItem(forTypeIdentifier: "public.url") { [weak self] url, _ in
                // Process the shared URL
                self?.extensionContext?.completeRequest(returningItems: [], completionHandler: nil)
            }
        }
    }

    override func configurationItems() -> [Any]! {
        return []  // Return SLComposeSheetConfigurationItem array for extra UI rows
    }
}
```

## Info.plist Configuration

Set these keys in the extension target's Info.plist (via XcodeGen):

```
NSExtensionPrincipalClass: $(PRODUCT_MODULE_NAME).ShareViewController
NSExtensionActivationRule:
  NSExtensionActivationSupportsWebURLWithMaxCount: 1
  NSExtensionActivationSupportsText: true
```

## Data Sharing via App Groups

Use the App Group entitlement to share data between the main app and the extension. Both targets must belong to the same App Group to access shared `UserDefaults` or a shared container directory.
