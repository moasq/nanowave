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
    @State private var manager = SubscriptionManager.shared

    var body: some View {
        NavigationStack {
            ZStack {
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

                if manager.purchaseSuccess {
                    purchaseSuccessOverlay
                }
            }
        }
        .task { await manager.loadOfferings() }
        .onChange(of: manager.purchaseSuccess) { _, success in
            if success {
                Task {
                    try? await Task.sleep(for: .seconds(1.5))
                    manager.resetPurchaseSuccess()
                    dismiss()
                }
            }
        }
        .alert("Error", isPresented: .constant(manager.errorMessage != nil)) {
            Button("OK") { manager.errorMessage = nil }
        } message: {
            Text(manager.errorMessage ?? "")
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
        ForEach(manager.packages, id: \.identifier) { package in
            PaywallPlanCard(
                package: package,
                isSelected: manager.selectedPackage?.identifier == package.identifier,
                onTap: { manager.selectedPackage = package }
            )
        }
    }
}
```

## CTA and Footer

```swift
private var ctaButton: some View {
    Button {
        guard let pkg = manager.selectedPackage else { return }
        Task { await manager.purchase(pkg) }
    } label: {
        if manager.isLoading {
            ProgressView()
        } else {
            Text("Subscribe")
                .font(AppTheme.Fonts.headline)
        }
    }
    .buttonStyle(.borderedProminent)
    .disabled(manager.selectedPackage == nil || manager.isLoading)
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
            Task { await manager.restore() }
        }
        .font(AppTheme.Fonts.caption)
    }
    .multilineTextAlignment(.center)
}
```

## Purchase Success Overlay

```swift
private var purchaseSuccessOverlay: some View {
    ZStack {
        Color.black.opacity(0.6)
            .ignoresSafeArea()

        VStack(spacing: AppTheme.Spacing.md) {
            Image(systemName: "checkmark.circle.fill")
                .font(.system(size: 64))
                .foregroundStyle(AppTheme.Colors.success)
                .symbolEffect(.bounce, value: manager.purchaseSuccess)

            Text("You're all set!")
                .font(AppTheme.Fonts.title2)
                .foregroundStyle(.white)

            Text("Your premium access is now active")
                .font(AppTheme.Fonts.body)
                .foregroundStyle(.white.opacity(0.8))
        }
    }
    .transition(.opacity)
    .animation(.easeInOut(duration: 0.3), value: manager.purchaseSuccess)
}
```
