package revenuecat

import (
	"context"
	"fmt"
	"strings"

	"github.com/moasq/nanowave/internal/integrations"
)

// PromptContribution generates the RevenueCat integration prompt content.
func (r *revenuecatProvider) PromptContribution(_ context.Context, req integrations.PromptRequest) (*integrations.PromptContribution, error) {
	cfg, _ := req.Store.GetProvider(integrations.ProviderRevenueCat, req.AppName)

	var system strings.Builder
	system.WriteString("\n<revenuecat-config>\n")

	// Resolve real values from config and plan
	apiKey := "YOUR_REVENUECAT_API_KEY"
	if cfg != nil && cfg.AnonKey != "" {
		apiKey = cfg.AnonKey
	}

	entitlementID := "premium"
	if req.MonetizationPlan != nil && req.MonetizationPlan.Entitlement != "" {
		entitlementID = req.MonetizationPlan.Entitlement
	}

	// Emit the exact AppConfig.swift the builder MUST create
	system.WriteString("## AppConfig.swift (REQUIRED — create this file exactly)\n\n")
	system.WriteString("```swift\n")
	system.WriteString("import Foundation\n\n")
	system.WriteString("enum AppConfig {\n")
	fmt.Fprintf(&system, "    static let revenueCatAPIKey = %q\n", apiKey)
	fmt.Fprintf(&system, "    static let entitlementID = %q\n", entitlementID)

	// Emit product identifiers as constants
	if req.MonetizationPlan != nil && len(req.MonetizationPlan.Products) > 0 {
		system.WriteString("\n    enum ProductID {\n")
		for _, p := range req.MonetizationPlan.Products {
			constName := productIdentifierToConstName(p.Identifier)
			fmt.Fprintf(&system, "        static let %s = %q\n", constName, p.Identifier)
		}
		system.WriteString("    }\n")
	}

	system.WriteString("}\n")
	system.WriteString("```\n\n")

	// SDK initialization
	system.WriteString("## SDK Initialization (in App.init())\n\n")
	system.WriteString("```swift\n")
	system.WriteString("import RevenueCat\n\n")
	system.WriteString("// In your @main App struct init():\n")
	system.WriteString("#if DEBUG\n")
	system.WriteString("Purchases.logLevel = .debug\n")
	system.WriteString("#endif\n")
	system.WriteString("Purchases.configure(\n")
	system.WriteString("    with: Configuration.Builder(withAPIKey: AppConfig.revenueCatAPIKey)\n")
	system.WriteString("        .with(storeKitVersion: .storeKit2)\n")
	system.WriteString("        .build()\n")
	system.WriteString(")\n")
	system.WriteString("```\n\n")

	// Product configuration from monetization plan
	if req.MonetizationPlan != nil && len(req.MonetizationPlan.Products) > 0 {
		system.WriteString("## Products (provisioned in RevenueCat)\n\n")
		fmt.Fprintf(&system, "Entitlement ID: %q\n\n", entitlementID)

		system.WriteString("| Identifier | Type | Display Name | Duration |\n")
		system.WriteString("|---|---|---|---|\n")
		for _, p := range req.MonetizationPlan.Products {
			dur := "-"
			if p.Duration != "" {
				dur = p.Duration
			}
			fmt.Fprintf(&system, "| %s | %s | %s | %s |\n", p.Identifier, p.Type, p.DisplayName, dur)
		}
		system.WriteString("\n")
	}

	// ==========================================
	// MANDATORY ARCHITECTURE — exact code patterns the builder MUST follow
	// ==========================================
	system.WriteString(`## MANDATORY ARCHITECTURE — Follow These Patterns Exactly

### BANNED: Do NOT create any of these
- Do NOT create a SubscriptionPlan / SubscriptionTier / Plan enum or struct with price properties
- Do NOT create a model that maps product identifiers to hardcoded "$X.XX" price strings
- Do NOT create sampleData or mock plans with price strings
- Do NOT use a ViewModel that holds a [SubscriptionPlan] array with hardcoded plans
- Do NOT display prices from any source other than package.storeProduct.localizedPriceString
- Do NOT create a PaywallViewModel — PaywallView uses SubscriptionManager.shared directly (no intermediary ViewModel)
- Do NOT call Purchases.shared from any file other than SubscriptionManager
- Do NOT present PaywallView with .sheet — MUST use .fullScreenCover
- Do NOT hardcode savings percentages (e.g., "Save ~17%") — calculate from package.storeProduct.price

### REQUIRED: SubscriptionManager Pattern

The SubscriptionManager holds RevenueCat Package objects directly. No intermediary plan model.

` + "```swift\n" + `import Foundation
import RevenueCat

@Observable
@MainActor
final class SubscriptionManager {
    static let shared = SubscriptionManager()

    var isPremium = false
    var packages: [Package] = []        // <-- RevenueCat Package objects, NOT custom plan models
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
            if selectedPackage == nil {
                selectedPackage = packages.first
            }
        } catch {
            errorMessage = "Could not load plans. Please try again."
        }
    }

    func purchase(_ package: Package) async {      // <-- takes Package, NOT a custom plan enum
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
            if !isPremium {
                errorMessage = "No active subscriptions found."
            }
        } catch {
            errorMessage = error.localizedDescription
        }
    }

    private func refreshStatus() async {
        do {
            let info = try await Purchases.shared.customerInfo()
            isPremium = info.entitlements[AppConfig.entitlementID]?.isActive == true
        } catch {
            errorMessage = error.localizedDescription
        }
    }

    private func listenForChanges() async {
        for try await info in Purchases.shared.customerInfoStream {
            isPremium = info.entitlements[AppConfig.entitlementID]?.isActive == true
        }
    }
}
` + "```\n\n" + `### REQUIRED: PaywallView Pattern

The PaywallView iterates SubscriptionManager.packages directly. Prices come from the store.

` + "```swift\n" + `struct PaywallView: View {
    @Environment(\.dismiss) private var dismiss
    @State private var manager = SubscriptionManager.shared

    var body: some View {
        ScrollView {
            VStack(spacing: AppTheme.Spacing.lg) {
                // Hero section ...
                // Plan cards — iterate Package objects, NOT custom models
                ForEach(manager.packages, id: \.identifier) { package in
                    PlanCard(
                        package: package,
                        isSelected: manager.selectedPackage?.identifier == package.identifier,
                        onTap: { manager.selectedPackage = package }
                    )
                }
                // CTA button
                Button {
                    guard let pkg = manager.selectedPackage else { return }
                    Task { await manager.purchase(pkg) }
                } label: {
                    Text("Subscribe")
                        .font(AppTheme.Fonts.headline)
                }
                // Footer with restore, terms, privacy ...
            }
        }
        .task { await manager.loadOfferings() }
    }
}

struct PlanCard: View {
    let package: Package
    let isSelected: Bool
    let onTap: () -> Void

    var body: some View {
        Button(action: onTap) {
            VStack(alignment: .leading, spacing: AppTheme.Spacing.sm) {
                Text(package.storeProduct.localizedTitle)          // "Basic Monthly" — from store
                    .font(AppTheme.Fonts.headline)
                Text(package.storeProduct.localizedPriceString)    // "$2.99" — from store, dynamic
                    .font(AppTheme.Fonts.title2)
                Text(package.storeProduct.localizedDescription)    // from store
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
` + "```\n\n" + `### Key Rules

1. The ONLY data source for plan names and prices is RevenueCat Package objects
2. package.storeProduct.localizedPriceString for prices (NEVER "$X.XX" strings)
3. package.storeProduct.localizedTitle for plan names
4. SubscriptionManager.purchase() takes a Package parameter, NOT a custom enum
5. PaywallView iterates manager.packages (type: [Package]) from RevenueCat
6. No SubscriptionPlan enum, no Plan struct, no PricingTier — only Package from RevenueCat
7. You may add UI features (icons, badge labels, feature lists) as separate data NOT tied to pricing

`)

	// Apple compliance
	system.WriteString(`## Apple Compliance (mandatory, post-Jan 2026)

- Close button must be immediately visible (no cooldown timer)
- Full billed amount as most prominent price (minimum 16pt font)
- No toggles for plan selection — use tappable cards
- No fake urgency (countdown timers, "limited time" text)
- Schedule 2, Section 3.8(b) disclosure in paywall footer
- Terms of Service and Privacy Policy as tappable in-app links
- Restore Purchases button visible without scrolling
`)

	system.WriteString("</revenuecat-config>\n")

	// User message block
	var userBlock string
	hasMCP := cfg != nil && cfg.PAT != ""
	if hasMCP {
		if req.BackendProvisioned {
			userBlock = `REVENUECAT BACKEND (already provisioned):
Products, entitlements, and offerings are configured in RevenueCat.
CRITICAL: Use RevenueCat Package objects for ALL pricing data — NEVER create custom plan enums with price strings.
Prices MUST come from package.storeProduct.localizedPriceString. Follow the SubscriptionManager and PaywallView patterns in <revenuecat-config> exactly.

`
		} else {
			userBlock = `REVENUECAT SETUP (MCP available):
Use RevenueCat MCP tools to verify products and offerings are configured before writing Swift code.
CRITICAL: Use RevenueCat Package objects for ALL pricing data — NEVER create custom plan enums with price strings.

`
		}
	}

	return &integrations.PromptContribution{
		SystemBlock:        system.String(),
		UserBlock:          userBlock,
		BackendProvisioned: req.BackendProvisioned,
	}, nil
}

// productIdentifierToConstName converts "premium_monthly" to "premiumMonthly".
func productIdentifierToConstName(identifier string) string {
	parts := strings.Split(identifier, "_")
	if len(parts) <= 1 {
		return identifier
	}
	var b strings.Builder
	b.WriteString(parts[0])
	for _, part := range parts[1:] {
		if len(part) > 0 {
			b.WriteString(strings.ToUpper(part[:1]))
			b.WriteString(part[1:])
		}
	}
	return b.String()
}
