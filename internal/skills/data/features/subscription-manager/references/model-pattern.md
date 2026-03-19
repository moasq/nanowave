# Subscription Tier Model — Dynamic Pricing

## CRITICAL: No Hardcoded Prices

NEVER put dollar amounts, price strings, or numeric prices in a SubscriptionTier enum or model.
Prices MUST always come from RevenueCat/StoreKit at runtime via `package.storeProduct.localizedPriceString`.
Product identifiers come from `AppConfig.ProductID` constants.

## Correct Pattern — Packages Drive Everything

Do NOT create a SubscriptionTier enum with prices. Instead, iterate directly over RevenueCat packages:

```swift
// In SubscriptionManager or PaywallView:
let offerings = try await Purchases.shared.offerings()
let packages = offerings.current?.availablePackages ?? []

// Display each package with dynamic data from the store:
ForEach(packages, id: \.identifier) { package in
    PlanCard(
        title: package.storeProduct.localizedTitle,
        price: package.storeProduct.localizedPriceString,
        description: package.storeProduct.localizedDescription
    )
}
```

## If You Need a Tier Enum

Only create a tier enum for feature gating — never for pricing:

```swift
enum SubscriptionTier: String {
    case free
    case premium

    /// Resolve this tier from customer info — prices come from offerings, not here.
    static func current(from customerInfo: CustomerInfo) -> SubscriptionTier {
        if customerInfo.entitlements[AppConfig.entitlementID]?.isActive == true {
            return .premium
        }
        return .free
    }
}
```

## Forbidden

```swift
// WRONG — never do this
var priceLabel: String {
    switch self {
    case .basic: "$2.99/mo"   // hardcoded price
    case .pro: "$9.99/mo"     // hardcoded price
    }
}

// WRONG — never do this
static func fallbackPlans() -> [Plan] {
    [Plan(name: "Basic", price: "$2.99")]  // hardcoded
}
```

## Savings Badge

Calculate savings dynamically from package prices:

```swift
func savingsPercentage(monthly: Package, yearly: Package) -> Int {
    let monthlyPrice = monthly.storeProduct.price as Decimal
    let yearlyPrice = yearly.storeProduct.price as Decimal
    let annualizedMonthly = monthlyPrice * 12
    guard annualizedMonthly > 0 else { return 0 }
    let savings = 1.0 - NSDecimalNumber(decimal: yearlyPrice / annualizedMonthly).doubleValue
    return Int(savings * 100)
}
```
