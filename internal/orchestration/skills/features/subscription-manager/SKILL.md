---
name: "subscription-manager"
description: "Subscription and credit management architecture with RevenueCat. Use when implementing subscription state, credit balance, or feature gating."
---
# Subscription Manager

## Architecture

Single-layer: **SubscriptionManager** is an `@Observable @MainActor` singleton that wraps `Purchases.shared` directly. It holds RevenueCat `Package` objects — NOT custom plan models.

## BANNED Patterns

Do NOT create ANY of these:
- `enum SubscriptionPlan` / `enum SubscriptionTier` / `struct Plan` with price properties
- Models with `var price: String` returning "$X.XX"
- `static let sampleData: [SomePlanType]` with hardcoded plans
- ViewModels holding `[SubscriptionPlan]` or similar custom plan arrays
- Any type that maps product identifiers to dollar amount strings

## SubscriptionManager (REQUIRED Pattern)

```swift
import Foundation
import RevenueCat

@Observable
@MainActor
final class SubscriptionManager {
    static let shared = SubscriptionManager()

    var isPremium = false
    var packages: [Package] = []     // RevenueCat Package objects ONLY
    var selectedPackage: Package?
    var isLoading = false
    var errorMessage: String?

    private init() {}

    func configure() {
        #if DEBUG
        Purchases.logLevel = .debug
        #endif
        Purchases.configure(
            with: Configuration.Builder(withAPIKey: AppConfig.revenueCatAPIKey)
                .with(storeKitVersion: .storeKit2)
                .build()
        )
        Task { await refreshStatus() }
        Task { await listenForChanges() }
    }

    func loadOfferings() async {
        isLoading = true
        defer { isLoading = false }
        do {
            let offerings = try await Purchases.shared.offerings()
            packages = offerings.current?.availablePackages ?? []
            if selectedPackage == nil { selectedPackage = packages.first }
        } catch {
            errorMessage = "Could not load plans."
        }
    }

    func purchase(_ package: Package) async {
        isLoading = true
        defer { isLoading = false }
        do {
            let result = try await Purchases.shared.purchase(package: package)
            if !result.userCancelled {
                isPremium = result.customerInfo.entitlements[AppConfig.entitlementID]?.isActive == true
            }
        } catch {
            errorMessage = error.localizedDescription
        }
    }

    func restore() async {
        isLoading = true
        defer { isLoading = false }
        do {
            let info = try await Purchases.shared.restorePurchases()
            isPremium = info.entitlements[AppConfig.entitlementID]?.isActive == true
            if !isPremium { errorMessage = "No active subscriptions found." }
        } catch {
            errorMessage = error.localizedDescription
        }
    }

    private func refreshStatus() async {
        do {
            let info = try await Purchases.shared.customerInfo()
            isPremium = info.entitlements[AppConfig.entitlementID]?.isActive == true
        } catch {}
    }

    private func listenForChanges() async {
        for try await info in Purchases.shared.customerInfoStream {
            isPremium = info.entitlements[AppConfig.entitlementID]?.isActive == true
        }
    }
}
```

Key points:
- `packages` is `[Package]` from RevenueCat — NOT a custom plan array
- `purchase()` takes a `Package` — NOT a custom enum
- Prices are NEVER stored in the manager — they come from `package.storeProduct.localizedPriceString` in the view layer
- Always use `AppConfig.entitlementID` — never hardcode "premium"

## Feature Gating

```swift
if SubscriptionManager.shared.isPremium {
    // Premium feature
} else {
    // Show paywall
}
```

## CRITICAL Rules

- ALL pricing data comes from RevenueCat Package objects at runtime
- NEVER put dollar amounts in enums, models, or fallback data
- NEVER hardcode entitlement names — use `AppConfig.entitlementID`
- If offerings haven't loaded yet, show a loading indicator — not a placeholder price
- Product identifiers come from `AppConfig.ProductID` constants

## Single Owner: SubscriptionManager

SubscriptionManager is the ONLY type that calls `Purchases.shared`. No other file should import and call `Purchases.shared` methods.

BANNED:
- `PaywallViewModel` or any ViewModel that wraps/duplicates SubscriptionManager
- Any View or ViewModel calling `Purchases.shared.purchase()`, `Purchases.shared.offerings()`, or `Purchases.shared.restorePurchases()` directly
- PaywallView must use `@State private var manager = SubscriptionManager.shared` — not its own ViewModel

See [Model Pattern](references/model-pattern.md) for why custom tier enums are banned.
