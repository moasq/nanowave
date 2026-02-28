---
name: "paywall"
description: "Custom paywall UI patterns for subscriptions and credit packs with Apple compliance. Use when building paywalls or purchase screens."
---
# Custom Paywall

Build custom paywalls matching the app's design system. DO NOT use RevenueCat's built-in `PaywallView` — always build custom UI.

## BANNED Patterns

Do NOT create:
- `SubscriptionPlan` / `SubscriptionTier` / `Plan` enum or struct with hardcoded prices
- Models with `var price: String` returning "$X.XX"
- ViewModels holding `[SubscriptionPlan]` — hold `[Package]` from RevenueCat instead
- Any fallback or sample data with price strings
- `PaywallViewModel` or any ViewModel that calls `Purchases.shared` — PaywallView uses `SubscriptionManager.shared` directly
- `.sheet` for paywall presentation — MUST use `.fullScreenCover`
- Hardcoded savings percentages ("Save ~17%") — must be calculated from StoreKit prices

## Data Source: RevenueCat Package Objects

The PaywallView gets its data from `SubscriptionManager.packages` which holds RevenueCat `Package` objects. ALL pricing comes from `package.storeProduct.localizedPriceString`. ALL plan names come from `package.storeProduct.localizedTitle`.

## PaywallView Pattern (REQUIRED)

```swift
struct PaywallView: View {
    @Environment(\.dismiss) private var dismiss
    @State private var manager = SubscriptionManager.shared

    var body: some View {
        ScrollView {
            VStack(spacing: AppTheme.Spacing.lg) {
                closeButton
                heroSection
                planCards
                ctaButton
                footer
            }
            .padding(AppTheme.Spacing.md)
        }
        .task { await manager.loadOfferings() }
    }

    private var planCards: some View {
        VStack(spacing: AppTheme.Spacing.sm) {
            ForEach(manager.packages, id: \.identifier) { package in
                PaywallPlanCard(
                    package: package,
                    isSelected: manager.selectedPackage?.identifier == package.identifier,
                    onTap: { manager.selectedPackage = package }
                )
            }
        }
    }

    private var ctaButton: some View {
        Button {
            guard let pkg = manager.selectedPackage else { return }
            Task { await manager.purchase(pkg) }
        } label: {
            Text("Subscribe")
                .font(AppTheme.Fonts.headline)
        }
        .buttonStyle(.borderedProminent)
        .disabled(manager.selectedPackage == nil || manager.isLoading)
    }
}
```

## Plan Card Pattern

```swift
struct PaywallPlanCard: View {
    let package: Package        // RevenueCat Package — NOT a custom model
    let isSelected: Bool
    let onTap: () -> Void

    var body: some View {
        Button(action: onTap) {
            VStack(alignment: .leading, spacing: AppTheme.Spacing.sm) {
                Text(package.storeProduct.localizedTitle)           // from store
                    .font(AppTheme.Fonts.headline)
                Text(package.storeProduct.localizedPriceString)     // from store
                    .font(AppTheme.Fonts.title2)
                Text(package.storeProduct.localizedDescription)     // from store
                    .font(AppTheme.Fonts.subheadline)
            }
            .padding(AppTheme.Spacing.md)
            .background(isSelected ? AppTheme.Colors.primary.opacity(0.1) : AppTheme.Colors.surface)
            .cornerRadius(AppTheme.Style.cornerRadius)
            .overlay(
                RoundedRectangle(cornerRadius: AppTheme.Style.cornerRadius)
                    .stroke(isSelected ? AppTheme.Colors.primary : .clear, lineWidth: 2)
            )
        }
        .buttonStyle(.plain)
    }
}
```

## Savings Badge (REQUIRED for multi-duration plans)

When showing annual alongside monthly plans, calculate savings dynamically from StoreKit prices. NEVER hardcode savings percentages.

```swift
// REQUIRED calculation — in the PlanCard or a helper
private var savingsText: String? {
    guard let monthlyPackage = manager.packages.first(where: { $0.packageType == .monthly }),
          let annualPackage = manager.packages.first(where: { $0.packageType == .annual }) else {
        return nil
    }
    let monthlyAnnualized = monthlyPackage.storeProduct.price * 12
    let annualPrice = annualPackage.storeProduct.price
    guard monthlyAnnualized > annualPrice else { return nil }
    let savings = ((monthlyAnnualized - annualPrice) / monthlyAnnualized * 100)
        .formatted(.number.precision(.fractionLength(0)))
    return "Save \(savings)%"
}
```

BANNED:
- Hardcoded savings strings like "Save ~17%", "Save 50%"
- Savings percentages that don't come from a calculation of actual StoreKit prices

## Apple Compliance (mandatory, post-Jan 2026)

Every paywall MUST follow these rules:

1. **Close button immediately visible** — no cooldown timer
2. **Full billed amount most prominent** — minimum 16pt font
3. **No toggles** — use tappable cards
4. **No fake urgency** — no countdown timers
5. **Schedule 2, Section 3.8(b) disclosure** in footer
6. **Terms of Service link** — tappable in-app link
7. **Privacy Policy link** — tappable in-app link
8. **Restore Purchases button** — visible without scrolling
9. **Dynamic pricing** — from `package.storeProduct.localizedPriceString`, never hardcoded
10. **Trial timeline** — show exact dates if offering trial

## Presentation — MUST use .fullScreenCover

Present paywalls as `.fullScreenCover`, NEVER as `.sheet`:

```swift
// REQUIRED
.fullScreenCover(isPresented: $showPaywall) {
    PaywallView()
}

// BANNED — never use .sheet for paywalls
.sheet(isPresented: $showPaywall) {    // WRONG
    PaywallView()
}
```

Why: `.sheet` allows swipe-to-dismiss which bypasses mandatory disclosures. Apple requires the close button to be the only dismissal mechanism so users see compliance text.

See [Compliance Checklist](references/compliance-checklist.md), [Subscription Paywall](references/subscription-paywall.md), [Credit Paywall](references/credit-paywall.md), and [Disclosure Text](references/disclosure-text.md) for templates.
