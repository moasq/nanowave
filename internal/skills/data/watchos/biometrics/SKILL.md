---
name: "biometrics"
description: "watchOS authentication: wrist detection, optic ID, LAContext on watchOS. Use when implementing watchOS-specific patterns related to biometrics."
---
# Biometrics (watchOS)

AUTHENTICATION ON watchOS:
- No Face ID or Touch ID — Apple Watch uses wrist detection and optic ID
- `LAContext` is available but `biometryType` returns `.opticID` on Apple Watch Ultra or `.none`
- Primary auth model: watch unlocks when iPhone is nearby + wrist detection

WRIST DETECTION:
- Watch locks automatically when removed from wrist
- `WKApplication.shared().isAutorotating` — not for auth, but useful context
- Auth state is implicitly managed by watchOS — if the watch is on wrist and unlocked, the user is authenticated

LAContext ON watchOS:
```swift
import LocalAuthentication

func authenticate() async -> Bool {
    let context = LAContext()
    var error: NSError?

    guard context.canEvaluatePolicy(.deviceOwnerAuthentication, error: &error) else {
        return false
    }

    do {
        return try await context.evaluatePolicy(
            .deviceOwnerAuthentication,
            localizedReason: "Authenticate to access sensitive data"
        )
    } catch {
        return false
    }
}
```

RULES:
- Use `.deviceOwnerAuthentication` (not `.deviceOwnerAuthenticationWithBiometrics`) — allows passcode fallback
- Do NOT check for `.faceID` or `.touchID` — they don't exist on watchOS
- Wrist detection is the primary security boundary — leverage it rather than prompting repeatedly
- For sensitive data, use `LAContext` to confirm the user's identity
- No `NSFaceIDUsageDescription` needed on watchOS
