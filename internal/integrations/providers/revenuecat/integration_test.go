package revenuecat

import (
	"context"
	"encoding/json"
	"os"
	"testing"
)

// These integration tests run against the real RevenueCat API.
// Set REVENUECAT_TEST_SK to a valid sk_ key to enable them.
// They validate that every API call in setup_helpers.go works correctly.

const envKey = "REVENUECAT_TEST_SK"

func testClient(t *testing.T) *rcClient {
	t.Helper()
	sk := os.Getenv(envKey)
	if sk == "" {
		t.Skipf("skipping integration test: %s not set", envKey)
	}
	return newRCClient(sk)
}

func TestIntegration_ListProjects(t *testing.T) {
	c := testClient(t)
	ctx := context.Background()

	projects, err := c.listProjects(ctx)
	if err != nil {
		t.Fatalf("listProjects: %v", err)
	}
	if len(projects) == 0 {
		t.Fatal("expected at least one project")
	}
	t.Logf("found %d project(s): %s (%s)", len(projects), projects[0].Name, projects[0].ID)
}

func TestIntegration_ListApps(t *testing.T) {
	c := testClient(t)
	ctx := context.Background()

	projects, err := c.listProjects(ctx)
	if err != nil {
		t.Fatalf("listProjects: %v", err)
	}
	if len(projects) == 0 {
		t.Skip("no projects found")
	}

	apps, err := c.listApps(ctx, projects[0].ID)
	if err != nil {
		t.Fatalf("listApps: %v", err)
	}
	if len(apps) == 0 {
		t.Fatal("expected at least one app")
	}
	t.Logf("found %d app(s): %s (%s, type=%s)", len(apps), apps[0].Name, apps[0].ID, apps[0].Type)
}

func TestIntegration_GetPublicAPIKeys(t *testing.T) {
	c := testClient(t)
	ctx := context.Background()

	projects, _ := c.listProjects(ctx)
	if len(projects) == 0 {
		t.Skip("no projects")
	}
	apps, _ := c.listApps(ctx, projects[0].ID)
	if len(apps) == 0 {
		t.Skip("no apps")
	}

	keys, err := c.getPublicAPIKeys(ctx, projects[0].ID, apps[0].ID)
	if err != nil {
		t.Fatalf("getPublicAPIKeys: %v", err)
	}
	if len(keys) == 0 {
		t.Fatal("expected at least one public API key")
	}
	// Validate key format
	for _, k := range keys {
		if k.Key == "" {
			t.Error("key is empty")
		}
		t.Logf("key: %s... (env=%s)", k.Key[:10], k.Environment)
	}
}

func TestIntegration_ListProducts(t *testing.T) {
	c := testClient(t)
	ctx := context.Background()

	projects, _ := c.listProjects(ctx)
	if len(projects) == 0 {
		t.Skip("no projects")
	}

	raw, err := c.listProducts(ctx, projects[0].ID)
	if err != nil {
		t.Fatalf("listProducts: %v", err)
	}
	var resp struct {
		Items []rcProduct `json:"items"`
	}
	if err := json.Unmarshal(raw, &resp); err != nil {
		t.Fatalf("parse products: %v", err)
	}
	t.Logf("found %d product(s)", len(resp.Items))
	for _, p := range resp.Items {
		t.Logf("  %s (id=%s, store=%s)", p.StoreIdentifier, p.ID, p.AppID)
	}
}

func TestIntegration_ListEntitlements(t *testing.T) {
	c := testClient(t)
	ctx := context.Background()

	projects, _ := c.listProjects(ctx)
	if len(projects) == 0 {
		t.Skip("no projects")
	}

	raw, err := c.listEntitlements(ctx, projects[0].ID)
	if err != nil {
		t.Fatalf("listEntitlements: %v", err)
	}
	var resp struct {
		Items []rcEntitlement `json:"items"`
	}
	if err := json.Unmarshal(raw, &resp); err != nil {
		t.Fatalf("parse entitlements: %v", err)
	}
	t.Logf("found %d entitlement(s)", len(resp.Items))
	for _, e := range resp.Items {
		t.Logf("  %s (id=%s, key=%s)", e.DisplayName, e.ID, e.LookupKey)
	}
}

func TestIntegration_ListOfferings(t *testing.T) {
	c := testClient(t)
	ctx := context.Background()

	projects, _ := c.listProjects(ctx)
	if len(projects) == 0 {
		t.Skip("no projects")
	}

	raw, err := c.listOfferings(ctx, projects[0].ID)
	if err != nil {
		t.Fatalf("listOfferings: %v", err)
	}
	var resp struct {
		Items []rcOffering `json:"items"`
	}
	if err := json.Unmarshal(raw, &resp); err != nil {
		t.Fatalf("parse offerings: %v", err)
	}
	if len(resp.Items) == 0 {
		t.Fatal("expected at least one offering")
	}
	t.Logf("found %d offering(s): %s (key=%s)", len(resp.Items), resp.Items[0].DisplayName, resp.Items[0].LookupKey)
}

func TestIntegration_FindHelpers(t *testing.T) {
	c := testClient(t)
	ctx := context.Background()

	projects, _ := c.listProjects(ctx)
	if len(projects) == 0 {
		t.Skip("no projects")
	}
	pid := projects[0].ID
	apps, _ := c.listApps(ctx, pid)
	if len(apps) == 0 {
		t.Skip("no apps")
	}
	appID := apps[0].ID

	t.Run("findProductByStoreID_existing", func(t *testing.T) {
		p := c.findProductByStoreID(ctx, pid, appID, "basic_monthly")
		if p == nil {
			t.Fatal("expected to find product basic_monthly")
		}
		t.Logf("found: %s (id=%s)", p.StoreIdentifier, p.ID)
	})

	t.Run("findProductByStoreID_nonexistent", func(t *testing.T) {
		p := c.findProductByStoreID(ctx, pid, appID, "nonexistent_product_xyz")
		if p != nil {
			t.Fatalf("expected nil for nonexistent product, got %s", p.StoreIdentifier)
		}
	})

	t.Run("findOfferingByKey_existing", func(t *testing.T) {
		o := c.findOfferingByKey(ctx, pid, "default")
		if o == nil {
			t.Fatal("expected to find offering 'default'")
		}
		t.Logf("found: %s (id=%s)", o.LookupKey, o.ID)
	})

	t.Run("findOfferingByKey_nonexistent", func(t *testing.T) {
		o := c.findOfferingByKey(ctx, pid, "nonexistent_offering_xyz")
		if o != nil {
			t.Fatalf("expected nil for nonexistent offering, got %s", o.LookupKey)
		}
	})

	t.Run("findEntitlementByKey_existing", func(t *testing.T) {
		e := c.findEntitlementByKey(ctx, pid, "premium")
		if e == nil {
			t.Fatal("expected to find entitlement 'premium'")
		}
		t.Logf("found: %s (id=%s)", e.LookupKey, e.ID)
	})

	t.Run("findEntitlementByKey_nonexistent", func(t *testing.T) {
		e := c.findEntitlementByKey(ctx, pid, "nonexistent_entitlement_xyz")
		if e != nil {
			t.Fatalf("expected nil for nonexistent entitlement, got %s", e.LookupKey)
		}
	})
}

func TestIntegration_ValidateConnection(t *testing.T) {
	c := testClient(t)
	ctx := context.Background()

	projects, _ := c.listProjects(ctx)
	if len(projects) == 0 {
		t.Skip("no projects")
	}

	if err := c.validateConnection(ctx, projects[0].ID); err != nil {
		t.Fatalf("validateConnection: %v", err)
	}
	t.Log("connection validated successfully")
}

// TestIntegration_CreateAndCleanup tests the full create → find → reuse cycle
// that the provision flow uses. It creates a product, entitlement, offering,
// package, and attaches everything — then verifies the find helpers can locate
// them. Uses unique names to avoid conflicts.
// NOTE: This test makes many API calls and may hit rate limits on the free tier.
// Run with: go test -timeout 5m -run TestIntegration_CreateAndCleanup
func TestIntegration_CreateAndCleanup(t *testing.T) {
	c := testClient(t)
	ctx := context.Background()

	projects, _ := c.listProjects(ctx)
	if len(projects) == 0 {
		t.Skip("no projects")
	}
	pid := projects[0].ID
	apps, _ := c.listApps(ctx, pid)
	if len(apps) == 0 {
		t.Skip("no apps")
	}
	appID := apps[0].ID

	// Use a unique suffix to avoid 409 conflicts with previous test runs
	suffix := "integration_test"

	// 1. Create product
	t.Run("create_product", func(t *testing.T) {
		input := rcProductInput{
			StoreIdentifier: "test_product_" + suffix,
			AppID:           appID,
			Type:            "subscription",
			DisplayName:     "Test Product",
			Title:           "Test Product",
			Subscription:    &rcProductSubscriptionInput{Duration: "P1M"},
		}
		product, err := c.createProduct(ctx, pid, input)
		if err != nil {
			// 409 = already exists from previous run, that's fine
			t.Logf("create product (may already exist): %v", err)
			existing := c.findProductByStoreID(ctx, pid, appID, "test_product_"+suffix)
			if existing == nil {
				t.Fatal("product creation failed and not found")
			}
			t.Logf("found existing: %s", existing.ID)
			return
		}
		if product.ID == "" {
			t.Fatal("product ID is empty")
		}
		t.Logf("created product: %s (id=%s)", product.StoreIdentifier, product.ID)
	})

	// 2. Create entitlement
	t.Run("create_entitlement", func(t *testing.T) {
		ent, err := c.createEntitlement(ctx, pid, "test_ent_"+suffix, "Test Entitlement")
		if err != nil {
			t.Logf("create entitlement (may already exist): %v", err)
			existing := c.findEntitlementByKey(ctx, pid, "test_ent_"+suffix)
			if existing == nil {
				t.Fatal("entitlement creation failed and not found")
			}
			t.Logf("found existing: %s", existing.ID)
			return
		}
		if ent.ID == "" {
			t.Fatal("entitlement ID is empty")
		}
		t.Logf("created entitlement: %s (id=%s)", ent.LookupKey, ent.ID)
	})

	// 3. Attach product to entitlement
	t.Run("attach_product_to_entitlement", func(t *testing.T) {
		product := c.findProductByStoreID(ctx, pid, appID, "test_product_"+suffix)
		if product == nil {
			t.Skip("test product not found")
		}
		ent := c.findEntitlementByKey(ctx, pid, "test_ent_"+suffix)
		if ent == nil {
			t.Skip("test entitlement not found")
		}
		err := c.attachProductsToEntitlement(ctx, pid, ent.ID, []string{product.ID})
		if err != nil {
			// 409 = already attached
			t.Logf("attach (may already be attached): %v", err)
		} else {
			t.Log("attached product to entitlement")
		}
	})

	// 4. Create offering
	t.Run("create_offering", func(t *testing.T) {
		off, err := c.createOffering(ctx, pid, "test_off_"+suffix, "Test Offering")
		if err != nil {
			t.Logf("create offering (may already exist): %v", err)
			existing := c.findOfferingByKey(ctx, pid, "test_off_"+suffix)
			if existing == nil {
				t.Fatal("offering creation failed and not found")
			}
			t.Logf("found existing: %s", existing.ID)
			return
		}
		if off.ID == "" {
			t.Fatal("offering ID is empty")
		}
		t.Logf("created offering: %s (id=%s)", off.LookupKey, off.ID)
	})

	// 5. Create package in offering
	t.Run("create_package", func(t *testing.T) {
		off := c.findOfferingByKey(ctx, pid, "test_off_"+suffix)
		if off == nil {
			t.Skip("test offering not found")
		}
		pkg, err := c.createPackage(ctx, pid, off.ID, "test_pkg_"+suffix, "Test Package", 1)
		if err != nil {
			t.Logf("create package (may already exist): %v", err)
			return
		}
		if pkg.ID == "" {
			t.Fatal("package ID is empty")
		}
		t.Logf("created package: %s (id=%s)", pkg.LookupKey, pkg.ID)
	})

	// 6. Attach product to package (this was the failing call!)
	t.Run("attach_product_to_package", func(t *testing.T) {
		product := c.findProductByStoreID(ctx, pid, appID, "test_product_"+suffix)
		if product == nil {
			t.Skip("test product not found")
		}
		off := c.findOfferingByKey(ctx, pid, "test_off_"+suffix)
		if off == nil {
			t.Skip("test offering not found")
		}

		// Find the package - need to list packages in the offering
		raw, err := c.doJSON(ctx, "GET", "/projects/"+pid+"/offerings/"+off.ID+"/packages", nil)
		if err != nil {
			t.Fatalf("list packages: %v", err)
		}
		var pkgResp struct {
			Items []rcPackage `json:"items"`
		}
		if err := json.Unmarshal(raw, &pkgResp); err != nil {
			t.Fatalf("parse packages: %v", err)
		}
		var pkgID string
		for _, p := range pkgResp.Items {
			if p.LookupKey == "test_pkg_"+suffix {
				pkgID = p.ID
				break
			}
		}
		if pkgID == "" {
			t.Skip("test package not found in offering")
		}

		// THIS is the call that was failing with 400 before the eligibility_criteria fix
		err = c.attachProductToPackage(ctx, pid, pkgID, product.ID)
		if err != nil {
			t.Fatalf("attachProductToPackage FAILED: %v", err)
		}
		t.Log("SUCCESS: attached product to package with eligibility_criteria")
	})
}

// TestIntegration_FullProvisionFlow simulates the exact flow that provision.go uses.
// This validates the entire provisioning pipeline end-to-end.
func TestIntegration_FullProvisionFlow(t *testing.T) {
	c := testClient(t)
	ctx := context.Background()

	projects, _ := c.listProjects(ctx)
	if len(projects) == 0 {
		t.Skip("no projects")
	}
	pid := projects[0].ID
	apps, _ := c.listApps(ctx, pid)
	if len(apps) == 0 {
		t.Skip("no apps")
	}
	appID := apps[0].ID

	// Verify the nanowave-provisioned resources are correctly wired
	t.Run("verify_products_exist", func(t *testing.T) {
		for _, storeID := range []string{"basic_monthly", "pro_monthly", "premium_yearly"} {
			p := c.findProductByStoreID(ctx, pid, appID, storeID)
			if p == nil {
				t.Errorf("product %s not found", storeID)
			} else {
				t.Logf("✓ %s -> %s", storeID, p.ID)
			}
		}
	})

	t.Run("verify_entitlement_exists", func(t *testing.T) {
		e := c.findEntitlementByKey(ctx, pid, "premium")
		if e == nil {
			t.Fatal("entitlement 'premium' not found")
		}
		t.Logf("✓ premium -> %s", e.ID)
	})

	t.Run("verify_offering_exists", func(t *testing.T) {
		o := c.findOfferingByKey(ctx, pid, "default")
		if o == nil {
			t.Fatal("offering 'default' not found")
		}
		t.Logf("✓ default -> %s", o.ID)
	})

	t.Run("verify_packages_have_products", func(t *testing.T) {
		o := c.findOfferingByKey(ctx, pid, "default")
		if o == nil {
			t.Skip("offering not found")
		}

		raw, err := c.doJSON(ctx, "GET", "/projects/"+pid+"/offerings/"+o.ID+"/packages", nil)
		if err != nil {
			t.Fatalf("list packages: %v", err)
		}
		var pkgResp struct {
			Items []rcPackage `json:"items"`
		}
		if err := json.Unmarshal(raw, &pkgResp); err != nil {
			t.Fatalf("parse: %v", err)
		}

		// Check nanowave packages specifically
		nanowavePackages := map[string]bool{
			"basic_monthly":  false,
			"pro_monthly":    false,
			"premium_yearly": false,
		}

		for _, pkg := range pkgResp.Items {
			if _, isNanowave := nanowavePackages[pkg.LookupKey]; !isNanowave {
				continue
			}
			// Check this package has products
			prodRaw, err := c.doJSON(ctx, "GET", "/projects/"+pid+"/packages/"+pkg.ID+"/products", nil)
			if err != nil {
				t.Errorf("list products for package %s: %v", pkg.LookupKey, err)
				continue
			}
			var prodResp struct {
				Items []struct {
					EligibilityCriteria string `json:"eligibility_criteria"`
					Product             struct {
						StoreIdentifier string `json:"store_identifier"`
					} `json:"product"`
				} `json:"items"`
			}
			if err := json.Unmarshal(prodRaw, &prodResp); err != nil {
				t.Errorf("parse products for %s: %v", pkg.LookupKey, err)
				continue
			}
			if len(prodResp.Items) == 0 {
				t.Errorf("✗ package %s has NO products attached", pkg.LookupKey)
			} else {
				item := prodResp.Items[0]
				if item.EligibilityCriteria != "all" {
					t.Errorf("✗ package %s eligibility_criteria = %q (expected 'all')", pkg.LookupKey, item.EligibilityCriteria)
				}
				t.Logf("✓ %s -> %s (eligibility=%s)", pkg.LookupKey, item.Product.StoreIdentifier, item.EligibilityCriteria)
				nanowavePackages[pkg.LookupKey] = true
			}
		}

		for key, found := range nanowavePackages {
			if !found {
				t.Errorf("✗ nanowave package %s not found in offering", key)
			}
		}
	})
}
