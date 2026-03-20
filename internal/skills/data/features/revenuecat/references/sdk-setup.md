# SDK Setup

## Contents
- Configuration Builder
- API Key Management
- Debug Logging
- Initialization Timing

## Configuration Builder

RevenueCat uses a builder pattern for configuration:

```swift
Purchases.configure(
    with: Configuration.Builder(withAPIKey: AppConfig.revenueCatAPIKey)
        .with(storeKitVersion: .storeKit2)
        .with(appUserID: nil) // anonymous until logIn()
        .build()
)
```

Always use `.storeKit2` — it enables modern StoreKit APIs and transaction listeners.

## API Key Management

Store the key in `Config/AppConfig.swift` alongside other RevenueCat constants:

```swift
enum AppConfig {
    static let revenueCatAPIKey = "appl_xxxxxxxxxxxx" // real key injected by nanowave
    static let entitlementID = "premium"              // matches RevenueCat dashboard

    enum ProductID {
        static let premiumMonthly = "premium_monthly"
        static let premiumYearly = "premium_yearly"
    }
}
```

The real API key and product IDs are provided in `<revenuecat-config>`. Use those exact values.

## Debug Logging

Enable verbose logging in debug builds:

```swift
#if DEBUG
Purchases.logLevel = .debug
#endif
```

Log levels: `.debug`, `.info`, `.warn`, `.error`

## Initialization Timing

- Configure in `App.init()` before any view loads
- Do NOT configure in `onAppear` or `task` — it must happen once at launch
- `Purchases.shared` is available after `configure()` returns
