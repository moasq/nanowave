# Subscription Paywall Template

## Contents
- View Structure
- Feature List Section
- Plan Cards Section
- CTA and Footer

## View Structure

```swift
struct PaywallView: View {
    @Environment(\.dismiss) private var dismiss
    @State private var selectedPackage: Package?
    @State private var packages: [Package] = []
    @State private var isPurchasing = false
    @State private var errorMessage: String?

    var body: some View {
        NavigationStack {
            ScrollView {
                VStack(spacing: AppTheme.Spacing.lg) {
                    heroSection
                    featureList
                    planCards
                    ctaButton
                    footer
                }
                .padding(AppTheme.Spacing.md)
            }
            .toolbar {
                ToolbarItem(placement: .cancellationAction) {
                    Button("Close") { dismiss() }
                }
            }
        }
        .task { await loadOfferings() }
        .alert("Error", isPresented: .constant(errorMessage != nil)) {
            Button("OK") { errorMessage = nil }
        } message: {
            Text(errorMessage ?? "")
        }
    }
}
```

## Feature List Section

```swift
private var featureList: some View {
    VStack(alignment: .leading, spacing: AppTheme.Spacing.sm) {
        PaywallFeatureRow(icon: "star.fill", text: "Feature description")
        PaywallFeatureRow(icon: "bolt.fill", text: "Another feature")
        PaywallFeatureRow(icon: "infinity", text: "Unlimited access")
    }
}

struct PaywallFeatureRow: View {
    let icon: String
    let text: String

    var body: some View {
        HStack(spacing: AppTheme.Spacing.sm) {
            Image(systemName: icon)
                .foregroundStyle(AppTheme.Colors.primary)
            Text(text)
                .font(AppTheme.Fonts.body)
        }
    }
}
```

## Plan Cards Section

```swift
private var planCards: some View {
    VStack(spacing: AppTheme.Spacing.sm) {
        ForEach(packages, id: \.identifier) { package in
            PaywallPlanCard(
                package: package,
                isSelected: selectedPackage?.identifier == package.identifier,
                onTap: { selectedPackage = package }
            )
        }
    }
}
```

## CTA and Footer

```swift
private var ctaButton: some View {
    Button {
        Task { await purchase() }
    } label: {
        if isPurchasing {
            ProgressView()
        } else {
            Text("Subscribe")
                .font(AppTheme.Fonts.headline)
        }
    }
    .buttonStyle(.borderedProminent)
    .disabled(selectedPackage == nil || isPurchasing)
}

private var footer: some View {
    VStack(spacing: AppTheme.Spacing.xs) {
        Text("Payment will be charged to your Apple ID account at confirmation of purchase. Subscription automatically renews unless canceled at least 24 hours before the end of the current period. Your account will be charged for renewal within 24 hours prior to the end of the current period. You can manage and cancel your subscriptions by going to Settings > Apple ID > Subscriptions.")
            .font(AppTheme.Fonts.caption)
            .foregroundStyle(AppTheme.Colors.textTertiary)

        HStack(spacing: AppTheme.Spacing.md) {
            Link("Terms of Service", destination: URL(string: "https://example.com/terms")!)
            Link("Privacy Policy", destination: URL(string: "https://example.com/privacy")!)
        }
        .font(AppTheme.Fonts.caption)

        Button("Restore Purchases") {
            Task { await restore() }
        }
        .font(AppTheme.Fonts.caption)
    }
    .multilineTextAlignment(.center)
}

private func loadOfferings() async {
    do {
        let offerings = try await Purchases.shared.offerings()
        packages = offerings.current?.availablePackages ?? []
        selectedPackage = packages.first
    } catch {
        errorMessage = "Could not load plans. Please try again."
    }
}

private func purchase() async {
    guard let package = selectedPackage else { return }
    isPurchasing = true
    defer { isPurchasing = false }
    do {
        let result = try await Purchases.shared.purchase(package: package)
        if !result.userCancelled {
            dismiss()
        }
    } catch {
        errorMessage = error.localizedDescription
    }
}

private func restore() async {
    do {
        let info = try await Purchases.shared.restorePurchases()
        if info.entitlements[AppConfig.entitlementID]?.isActive == true {
            dismiss()
        } else {
            errorMessage = "No active subscriptions found."
        }
    } catch {
        errorMessage = error.localizedDescription
    }
}
```
