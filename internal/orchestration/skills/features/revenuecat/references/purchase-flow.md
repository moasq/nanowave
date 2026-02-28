# Purchase Flow

## Contents
- Fetching Offerings
- Purchasing a Package
- Error Handling
- Restore Purchases
- Transaction Lifecycle

## Fetching Offerings

Offerings group products into displayable sets:

```swift
func fetchOfferings() async throws -> [Package] {
    let offerings = try await Purchases.shared.offerings()
    guard let current = offerings.current else {
        return []
    }
    return current.availablePackages
}
```

Each `Package` contains a `StoreProduct` with:
- `localizedPriceString` — formatted price (e.g., "$9.99") — ALWAYS use this, never hardcode
- `localizedTitle` — product name
- `localizedDescription` — product description
- `subscriptionPeriod` — duration for subscriptions

## Purchasing a Package

```swift
func purchase(_ package: Package) async throws -> Bool {
    let result = try await Purchases.shared.purchase(package: package)
    if result.userCancelled { return false }
    return result.customerInfo.entitlements[AppConfig.entitlementID]?.isActive == true
}
```

## Error Handling

```swift
do {
    let result = try await Purchases.shared.purchase(package: pkg)
    if result.userCancelled { return }
    // success
} catch let error as RevenueCat.ErrorCode {
    switch error {
    case .purchaseCancelledError:
        break // user cancelled
    case .paymentPendingError:
        showAlert("Purchase pending approval")
    case .storeProblemError:
        showAlert("App Store error. Please try again.")
    case .purchaseNotAllowedError:
        showAlert("Purchases not allowed on this device")
    default:
        showAlert("Purchase failed: \(error.localizedDescription)")
    }
}
```

## Restore Purchases

Required by Apple — must be accessible without scrolling:

```swift
func restorePurchases() async throws -> Bool {
    let customerInfo = try await Purchases.shared.restorePurchases()
    return customerInfo.entitlements[AppConfig.entitlementID]?.isActive == true
}
```

## Transaction Lifecycle

RevenueCat handles all StoreKit 2 transaction listeners automatically.
No manual `Transaction.updates` listener needed — the SDK manages:
- Transaction verification
- Receipt validation
- Server-side processing
- Renewal tracking
