package revenuecat

import (
	"context"
	"fmt"
	"strings"

	"github.com/moasq/nanowave/internal/integrations"
)

// Provision auto-provisions RevenueCat resources: products, entitlements, offerings, packages.
func (r *revenuecatProvider) Provision(_ context.Context, req integrations.ProvisionRequest) (*integrations.ProvisionResult, error) {
	if req.PAT == "" || !req.NeedsMonetization || req.MonetizationPlan == nil {
		return &integrations.ProvisionResult{}, nil
	}

	result := &integrations.ProvisionResult{}
	client := newRCClient(req.PAT)
	ctx := context.Background()

	// For RevenueCat: ProjectURL = project ID, ProjectRef = app ID
	projectID := req.ProjectURL

	plan := req.MonetizationPlan

	// 1. Create products
	// RevenueCat auto-creates default products ("Monthly", "Yearly", "Lifetime") for new
	// Test Store apps. We prefix display names with the app name to avoid conflicts.
	var productIDs []string
	for _, p := range plan.Products {
		rcType := "subscription"
		if p.Type == "consumable" {
			rcType = "consumable"
		} else if p.Type == "non_consumable" {
			rcType = "non_consumable"
		}

		displayName := fmt.Sprintf("%s %s", req.AppName, p.DisplayName)

		input := rcProductInput{
			StoreIdentifier: p.Identifier,
			AppID:           req.ProjectRef,
			Type:            rcType,
			DisplayName:     displayName,
			Title:           displayName, // required for Test Store products
		}

		// For subscriptions, include the duration (required for Test Store)
		if rcType == "subscription" && p.Duration != "" {
			input.Subscription = &rcProductSubscriptionInput{Duration: p.Duration}
		}

		product, err := client.createProduct(ctx, projectID, input)
		if err != nil {
			if strings.Contains(err.Error(), "409") {
				// Product already exists — try store_identifier first, then display_name
				existing := client.findProductByStoreID(ctx, projectID, req.ProjectRef, p.Identifier)
				if existing == nil {
					existing = client.findProductByDisplayName(ctx, projectID, req.ProjectRef, displayName)
				}
				if existing != nil {
					productIDs = append(productIDs, existing.ID)
					continue
				}
			}
			result.Warnings = append(result.Warnings, fmt.Sprintf("Failed to create product %s: %v", p.Identifier, err))
			continue
		}
		productIDs = append(productIDs, product.ID)
	}

	if len(productIDs) == 0 {
		result.Warnings = append(result.Warnings, "No products were created — skipping entitlement/offering setup")
		return result, nil
	}

	// 2. Create entitlement (or reuse existing)
	entitlementName := plan.Entitlement
	if entitlementName == "" {
		entitlementName = "premium"
	}
	ent, err := client.createEntitlement(ctx, projectID, entitlementName, entitlementName)
	if err != nil {
		if strings.Contains(err.Error(), "409") {
			ent = client.findEntitlementByKey(ctx, projectID, entitlementName)
		}
		if ent == nil {
			result.Warnings = append(result.Warnings, fmt.Sprintf("Failed to create entitlement: %v", err))
		}
	}

	// 3. Attach products to entitlement
	if ent != nil && len(productIDs) > 0 {
		if err := client.attachProductsToEntitlement(ctx, projectID, ent.ID, productIDs); err != nil {
			// 409 = products already attached — not a problem
			if !strings.Contains(err.Error(), "409") {
				result.Warnings = append(result.Warnings, fmt.Sprintf("Failed to attach products to entitlement: %v", err))
			}
		}
	}

	// 4. Create offering (or reuse existing)
	offering, err := client.createOffering(ctx, projectID, "default", "Default Offering")
	if err != nil {
		if strings.Contains(err.Error(), "409") {
			offering = client.findOfferingByKey(ctx, projectID, "default")
		}
		if offering == nil {
			result.Warnings = append(result.Warnings, fmt.Sprintf("Failed to create offering: %v", err))
			result.BackendProvisioned = len(productIDs) > 0
			return result, nil
		}
	}

	// 5. Create packages and attach products
	for i, p := range plan.Products {
		if i >= len(productIDs) {
			break
		}
		pkgDisplayName := fmt.Sprintf("%s %s", req.AppName, p.DisplayName)
		pkg, err := client.createPackage(ctx, projectID, offering.ID, p.Identifier, pkgDisplayName, i+1)
		if err != nil {
			if strings.Contains(err.Error(), "409") {
				// Package already exists — try lookup_key first, then display_name
				pkg = client.findPackageByKey(ctx, projectID, offering.ID, p.Identifier)
				if pkg == nil {
					pkg = client.findPackageByDisplayName(ctx, projectID, offering.ID, pkgDisplayName)
				}
				if pkg == nil {
					result.Warnings = append(result.Warnings, fmt.Sprintf("Package %s exists but could not be found", p.Identifier))
					continue
				}
			} else {
				result.Warnings = append(result.Warnings, fmt.Sprintf("Failed to create package for %s: %v", p.Identifier, err))
				continue
			}
		}
		if err := client.attachProductToPackage(ctx, projectID, pkg.ID, productIDs[i]); err != nil {
			// 409 = product already attached — not a problem
			if !strings.Contains(err.Error(), "409") {
				result.Warnings = append(result.Warnings, fmt.Sprintf("Failed to attach product to package %s: %v", p.Identifier, err))
			}
		}
	}

	result.BackendProvisioned = true
	return result, nil
}
