package supabase

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/moasq/nanowave/internal/integrations"
)

// Provision auto-provisions Supabase backend resources: auth, tables, RLS, storage, realtime.
// Delegates to the same Supabase Management API calls that were in pipeline.go.
func (s *supabaseProvider) Provision(_ context.Context, req integrations.ProvisionRequest) (*integrations.ProvisionResult, error) {
	if req.PAT == "" {
		return &integrations.ProvisionResult{}, nil
	}

	result := &integrations.ProvisionResult{}
	client := &apiClient{pat: req.PAT, projectRef: req.ProjectRef}

	// 1. Auth providers
	if req.NeedsAuth {
		authMethods := req.AuthMethods
		if len(authMethods) == 0 {
			authMethods = []string{"email", "anonymous"}
		}
		for _, m := range authMethods {
			if m == "apple" {
				result.NeedsAppleSignIn = true
				break
			}
		}
		if err := configureAuth(client, req.BundleID, authMethods); err != nil {
			result.Warnings = append(result.Warnings, fmt.Sprintf("Could not auto-configure auth providers: %v", err))
		}
	}

	// 2. Create tables from models
	if req.NeedsDB && len(req.Models) > 0 {
		sql := generateCreateTablesSQL(req.Models)
		if err := client.executeSQL(sql); err != nil {
			result.Warnings = append(result.Warnings, fmt.Sprintf("Table creation failed: %v", err))
		} else {
			result.BackendProvisioned = true
			for _, m := range req.Models {
				result.TablesCreated = append(result.TablesCreated, integrations.ModelRefTableName(m.Name))
			}
		}

		// 3. Enable RLS
		rlsSQL := generateEnableRLSSQL(req.Models)
		if err := client.executeSQL(rlsSQL); err != nil {
			result.Warnings = append(result.Warnings, fmt.Sprintf("RLS enable failed: %v", err))
		}

		// 4. Create RLS policies
		policySQL := generateRLSPoliciesSQL(req.Models)
		if err := client.executeSQL(policySQL); err != nil {
			result.Warnings = append(result.Warnings, fmt.Sprintf("RLS policies failed: %v", err))
		}
	}

	// 5. Storage bucket
	if req.NeedsStorage {
		bucketID := strings.ToLower(req.AppName) + "-media"
		bucketSQL := fmt.Sprintf(`INSERT INTO storage.buckets (id, name, public) VALUES ('%s', '%s', true) ON CONFLICT (id) DO NOTHING;`, bucketID, bucketID)
		if err := client.executeSQL(bucketSQL); err != nil {
			result.Warnings = append(result.Warnings, fmt.Sprintf("Storage bucket creation failed: %v", err))
		} else {
			policySQL := generateStoragePoliciesSQL(bucketID)
			if err := client.executeSQL(policySQL); err != nil {
				result.Warnings = append(result.Warnings, fmt.Sprintf("Storage policies failed: %v", err))
			}
		}
	}

	// 6. Realtime
	if req.NeedsRealtime && len(req.Models) > 0 {
		realtimeSQL := generateRealtimeSQL(req.Models)
		if err := client.executeSQL(realtimeSQL); err != nil {
			result.Warnings = append(result.Warnings, fmt.Sprintf("Realtime enable failed: %v", err))
		}
	}

	return result, nil
}

// --- Supabase Management API client (moved from pipeline.go) ---

type apiClient struct {
	pat        string
	projectRef string
}

func (c *apiClient) executeSQL(query string) error {
	data, err := json.Marshal(map[string]string{"query": query})
	if err != nil {
		return err
	}
	url := fmt.Sprintf("https://api.supabase.com/v1/projects/%s/database/query", c.projectRef)
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(data))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+c.pat)
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("SQL execution returned %d: %s", resp.StatusCode, string(body))
	}
	return nil
}

func (c *apiClient) updateAuthConfig(config map[string]any) error {
	data, err := json.Marshal(config)
	if err != nil {
		return err
	}
	url := fmt.Sprintf("https://api.supabase.com/v1/projects/%s/config/auth", c.projectRef)
	req, err := http.NewRequest(http.MethodPatch, url, bytes.NewReader(data))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+c.pat)
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("auth config update returned %d: %s", resp.StatusCode, string(body))
	}
	return nil
}

// --- Auth configuration ---

func configureAuth(c *apiClient, bundleID string, authMethods []string) error {
	config := make(map[string]any)
	config["mailer_autoconfirm"] = true
	for _, method := range authMethods {
		switch method {
		case "email":
			config["external_email_enabled"] = true
		case "anonymous":
			config["external_anonymous_users_enabled"] = true
		case "apple":
			config["external_apple_enabled"] = true
			if bundleID != "" {
				config["external_apple_client_id"] = bundleID
			}
		case "google":
			config["external_google_enabled"] = true
		case "phone":
			config["external_phone_enabled"] = true
		}
	}
	if len(config) == 0 {
		return nil
	}
	return c.updateAuthConfig(config)
}

// --- SQL generation (moved from pipeline.go) ---

func generateCreateTablesSQL(models []integrations.ModelRef) string {
	var b strings.Builder
	for _, m := range models {
		tableName := integrations.ModelRefTableName(m.Name)
		fmt.Fprintf(&b, "CREATE TABLE IF NOT EXISTS public.%s (\n", tableName)
		for i, prop := range m.Properties {
			colName := modelRefCamelToSnake(prop.Name)
			pgType := swiftTypeToPG(prop.Type)
			constraints := inferConstraints(integrations.PropertyRef(prop), i == 0, m.Name)
			fmt.Fprintf(&b, "  %s %s%s", colName, pgType, constraints)
			if i < len(m.Properties)-1 {
				b.WriteString(",")
			}
			b.WriteString("\n")
		}
		b.WriteString(");\n\n")
	}
	return b.String()
}

func generateEnableRLSSQL(models []integrations.ModelRef) string {
	var b strings.Builder
	for _, m := range models {
		fmt.Fprintf(&b, "ALTER TABLE public.%s ENABLE ROW LEVEL SECURITY;\n", integrations.ModelRefTableName(m.Name))
	}
	return b.String()
}

func generateRLSPoliciesSQL(models []integrations.ModelRef) string {
	var b strings.Builder
	for _, m := range models {
		tableName := integrations.ModelRefTableName(m.Name)
		hasUserID := false
		for _, prop := range m.Properties {
			if modelRefCamelToSnake(prop.Name) == "user_id" {
				hasUserID = true
				break
			}
		}
		policies := []struct{ suffix, op, clause string }{
			{"select", "SELECT", "USING (true)"},
		}
		if hasUserID {
			policies = append(policies,
				struct{ suffix, op, clause string }{"insert", "INSERT", "WITH CHECK (auth.uid() = user_id)"},
				struct{ suffix, op, clause string }{"update", "UPDATE", "USING (auth.uid() = user_id)"},
				struct{ suffix, op, clause string }{"delete", "DELETE", "USING (auth.uid() = user_id)"},
			)
		} else {
			policies = append(policies,
				struct{ suffix, op, clause string }{"insert", "INSERT", "WITH CHECK (auth.role() = 'authenticated')"},
				struct{ suffix, op, clause string }{"update", "UPDATE", "USING (auth.role() = 'authenticated')"},
				struct{ suffix, op, clause string }{"delete", "DELETE", "USING (auth.role() = 'authenticated')"},
			)
		}
		for _, p := range policies {
			policyName := fmt.Sprintf("%s_%s", tableName, p.suffix)
			fmt.Fprintf(&b, "DROP POLICY IF EXISTS \"%s\" ON public.%s;\n", policyName, tableName)
			fmt.Fprintf(&b, "CREATE POLICY \"%s\" ON public.%s FOR %s %s;\n", policyName, tableName, p.op, p.clause)
		}
		b.WriteString("\n")
	}
	return b.String()
}

func generateRealtimeSQL(models []integrations.ModelRef) string {
	var b strings.Builder
	for _, m := range models {
		tableName := integrations.ModelRefTableName(m.Name)
		fmt.Fprintf(&b, "ALTER PUBLICATION supabase_realtime ADD TABLE public.%s;\n", tableName)
		fmt.Fprintf(&b, "ALTER TABLE public.%s REPLICA IDENTITY FULL;\n", tableName)
	}
	return b.String()
}

func generateStoragePoliciesSQL(bucketID string) string {
	var b strings.Builder
	policies := []struct {
		suffix, op, clause string
	}{
		{
			"select", "SELECT",
			fmt.Sprintf("USING (bucket_id = '%s')", bucketID),
		},
		{
			"insert", "INSERT",
			fmt.Sprintf("WITH CHECK (bucket_id = '%s' AND auth.role() = 'authenticated' AND (storage.foldername(name))[1] = auth.uid()::text)", bucketID),
		},
		{
			"update", "UPDATE",
			fmt.Sprintf("USING (bucket_id = '%s' AND auth.uid()::text = (storage.foldername(name))[1])", bucketID),
		},
		{
			"delete", "DELETE",
			fmt.Sprintf("USING (bucket_id = '%s' AND auth.uid()::text = (storage.foldername(name))[1])", bucketID),
		},
	}
	for _, p := range policies {
		policyName := fmt.Sprintf("%s_%s", bucketID, p.suffix)
		fmt.Fprintf(&b, "DROP POLICY IF EXISTS \"%s\" ON storage.objects;\n", policyName)
		fmt.Fprintf(&b, "CREATE POLICY \"%s\" ON storage.objects FOR %s %s;\n", policyName, p.op, p.clause)
	}
	return b.String()
}
