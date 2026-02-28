# Entitlements

## Contents
- Checking Subscription Status
- Reactive Updates
- User Account Linking
- Credit / Consumable Tracking

## Checking Subscription Status

ALWAYS use `AppConfig.entitlementID` — never hardcode the entitlement name:

```swift
let customerInfo = try await Purchases.shared.customerInfo()
let isPremium = customerInfo.entitlements[AppConfig.entitlementID]?.isActive == true
```

The entitlement ID in `AppConfig` matches exactly what was configured in the RevenueCat dashboard.

## Reactive Updates

Use `customerInfoStream` for real-time subscription state:

```swift
Task {
    for try await customerInfo in Purchases.shared.customerInfoStream {
        isPremium = customerInfo.entitlements[AppConfig.entitlementID]?.isActive == true
    }
}
```

This fires when:
- A purchase completes
- A subscription renews or expires
- Purchases are restored
- The app returns to foreground

## User Account Linking

Link RevenueCat anonymous ID to your auth user:

```swift
// After successful login
let (customerInfo, created) = try await Purchases.shared.logIn(authUserId)

// On logout
let customerInfo = try await Purchases.shared.logOut()
```

- `logIn` transfers purchases to the identified user
- `logOut` generates a new anonymous ID
- Call `logIn` every time the user authenticates

## Credit / Consumable Tracking

For consumable credit packs, track balance locally or server-side:

```swift
// After purchasing a credit pack
let result = try await Purchases.shared.purchase(package: creditPack)
if !result.userCancelled {
    // Grant credits in your app's credit system
    creditManager.addCredits(pack.creditAmount)
}
```

RevenueCat tracks consumable purchases but does NOT track remaining credits.
Your app must maintain credit balance separately (e.g., @AppStorage, UserDefaults, or backend).
