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
    @State private var selectedPackage: Package?
    @State private var packages: [Package] = []
    @State private var isPurchasing = false
    let currentBalance: Int

    var body: some View {
        NavigationStack {
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
        }
        .task { await loadOfferings() }
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
        ForEach(packages, id: \.identifier) { package in
            CreditPackCard(
                package: package,
                isSelected: selectedPackage?.identifier == package.identifier,
                onTap: { selectedPackage = package }
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
    guard let package = selectedPackage else { return }
    isPurchasing = true
    defer { isPurchasing = false }

    do {
        let (_, customerInfo, cancelled) = try await Purchases.shared.purchase(package: package)
        if !cancelled {
            // Grant credits based on the product purchased
            dismiss()
        }
    } catch {
        // Show error
    }
}
```
