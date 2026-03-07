# Credit Pack Paywall Template

## Contents
- View Structure
- Credit Balance Display
- Credit Pack Cards
- Purchase Flow

## View Structure

```swift
struct CreditPaywallView: View {
    @Environment(\.dismiss) private var dismiss
    @State private var manager = SubscriptionManager.shared
    @State private var purchaseSuccess = false
    let currentBalance: Int

    var body: some View {
        NavigationStack {
            ZStack {
                ScrollView {
                    VStack(spacing: AppTheme.Spacing.lg) {
                        balanceSection
                        creditPacks
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

                if purchaseSuccess {
                    creditPurchaseSuccessOverlay
                }
            }
        }
        .task { await manager.loadOfferings() }
    }
}
```

## Credit Balance Display

```swift
private var balanceSection: some View {
    VStack(spacing: AppTheme.Spacing.sm) {
        Text("\(currentBalance)")
            .font(AppTheme.Fonts.largeTitle)
            .foregroundStyle(AppTheme.Colors.primary)
        Text("credits remaining")
            .font(AppTheme.Fonts.subheadline)
            .foregroundStyle(AppTheme.Colors.textSecondary)
    }
}
```

## Credit Pack Cards

```swift
private var creditPacks: some View {
    VStack(spacing: AppTheme.Spacing.sm) {
        ForEach(manager.packages, id: \.identifier) { package in
            CreditPackCard(
                package: package,
                isSelected: manager.selectedPackage?.identifier == package.identifier,
                onTap: { manager.selectedPackage = package }
            )
        }
    }
}

struct CreditPackCard: View {
    let package: Package
    let isSelected: Bool
    let onTap: () -> Void

    var body: some View {
        Button(action: onTap) {
            HStack {
                VStack(alignment: .leading, spacing: AppTheme.Spacing.xs) {
                    Text(package.storeProduct.localizedTitle)
                        .font(AppTheme.Fonts.headline)
                    Text(package.storeProduct.localizedDescription)
                        .font(AppTheme.Fonts.caption)
                        .foregroundStyle(AppTheme.Colors.textSecondary)
                }
                Spacer()
                Text(package.storeProduct.localizedPriceString)
                    .font(AppTheme.Fonts.title3)
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

## Purchase Flow

```swift
private func purchase() async {
    guard let package = manager.selectedPackage else { return }
    manager.isLoading = true
    defer { manager.isLoading = false }

    do {
        let result = try await Purchases.shared.purchase(package: package)
        if !result.userCancelled {
            // Grant credits based on the product purchased
            purchaseSuccess = true
            Task {
                try? await Task.sleep(for: .seconds(1.5))
                purchaseSuccess = false
                dismiss()
            }
        }
    } catch {
        manager.errorMessage = error.localizedDescription
    }
}
```

## Credit Purchase Success Overlay

```swift
private var creditPurchaseSuccessOverlay: some View {
    ZStack {
        Color.black.opacity(0.6)
            .ignoresSafeArea()

        VStack(spacing: AppTheme.Spacing.md) {
            Image(systemName: "checkmark.circle.fill")
                .font(.system(size: 64))
                .foregroundStyle(AppTheme.Colors.success)
                .symbolEffect(.bounce, value: purchaseSuccess)

            Text("Credits Added!")
                .font(AppTheme.Fonts.title2)
                .foregroundStyle(.white)

            Text("Your credits are ready to use")
                .font(AppTheme.Fonts.body)
                .foregroundStyle(.white.opacity(0.8))
        }
    }
    .transition(.opacity)
    .animation(.easeInOut(duration: 0.3), value: purchaseSuccess)
}
```
