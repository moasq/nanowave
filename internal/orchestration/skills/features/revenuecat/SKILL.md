---
name: "revenuecat"
description: "RevenueCat SDK patterns for in-app purchases, subscriptions, and entitlements. Use when implementing monetization features with RevenueCat."
---
# RevenueCat Integration

## SDK Initialization

Configure RevenueCat in your App's `init()` with StoreKit 2:

```swift
import RevenueCat

@main
struct MyApp: App {
    init() {
        #if DEBUG
        Purchases.logLevel = .debug
        #endif
        Purchases.configure(
            with: Configuration.Builder(withAPIKey: AppConfig.revenueCatAPIKey)
                .with(storeKitVersion: .storeKit2)
                .build()
        )
    }
}
```

The API key comes from `AppConfig.revenueCatAPIKey` — NEVER hardcode it inline.

## Fetching Offerings

```swift
let offerings = try await Purchases.shared.offerings()
if let current = offerings.current {
    for package in current.availablePackages {
        let product = package.storeProduct
        let price = product.localizedPriceString  // "$9.99" — dynamic from store
        let name = product.localizedTitle          // "Premium Monthly"
    }
}
```

## Purchasing

```swift
let result = try await Purchases.shared.purchase(package: package)
if !result.userCancelled {
    let isPremium = result.customerInfo.entitlements[AppConfig.entitlementID]?.isActive == true
}
```

Handle errors:
- `.purchaseCancelledError` — user tapped Cancel, do nothing
- `.paymentPendingError` — payment pending approval (Ask to Buy)
- `.storeProblemError` — App Store issue, show retry
- `.purchaseNotAllowedError` — device restrictions

## Checking Entitlements

ALWAYS use `AppConfig.entitlementID` — never hardcode the entitlement string:

```swift
let customerInfo = try await Purchases.shared.customerInfo()
let isPremium = customerInfo.entitlements[AppConfig.entitlementID]?.isActive == true
```

## Reactive Updates

```swift
for try await customerInfo in Purchases.shared.customerInfoStream {
    let isPremium = customerInfo.entitlements[AppConfig.entitlementID]?.isActive == true
}
```

## Restore Purchases

```swift
let customerInfo = try await Purchases.shared.restorePurchases()
```

## User Account Linking

When using authentication (Supabase, Firebase, etc.), link the user ID:

```swift
let (customerInfo, _) = try await Purchases.shared.logIn(userId)
```

Call `logIn` after the user authenticates. Call `logOut` on sign-out:

```swift
let customerInfo = try await Purchases.shared.logOut()
```

## CRITICAL: No Hardcoded Values

- API key: use `AppConfig.revenueCatAPIKey`
- Entitlement: use `AppConfig.entitlementID`
- Product IDs: use `AppConfig.ProductID` constants
- Prices: use `package.storeProduct.localizedPriceString` — NEVER hardcode dollar amounts

See [SDK Setup](references/sdk-setup.md), [Purchase Flow](references/purchase-flow.md), and [Entitlements](references/entitlements.md) for detailed patterns.
