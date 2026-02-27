package supabaseserver

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"
)

const managementAPIBase = "https://api.supabase.com"

// supabaseClient wraps HTTP calls to the Supabase Management API.
type supabaseClient struct {
	httpClient *http.Client
	pat        string // SUPABASE_ACCESS_TOKEN
	projectRef string // SUPABASE_PROJECT_REF
}

// newClientFromEnv reads credentials from environment variables.
func newClientFromEnv() (*supabaseClient, error) {
	pat := os.Getenv("SUPABASE_ACCESS_TOKEN")
	if pat == "" {
		return nil, fmt.Errorf("SUPABASE_ACCESS_TOKEN is not set")
	}
	ref := os.Getenv("SUPABASE_PROJECT_REF")
	if ref == "" {
		return nil, fmt.Errorf("SUPABASE_PROJECT_REF is not set")
	}
	return &supabaseClient{
		httpClient: &http.Client{Timeout: 30 * time.Second},
		pat:        pat,
		projectRef: ref,
	}, nil
}

func (c *supabaseClient) doJSON(ctx context.Context, method, path string, body any) (json.RawMessage, error) {
	url := managementAPIBase + path

	var reqBody io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("marshal request body: %w", err)
		}
		reqBody = bytes.NewReader(data)
	}

	req, err := http.NewRequestWithContext(ctx, method, url, reqBody)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+c.pat)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	respData, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("API %s %s returned %d: %s", method, path, resp.StatusCode, string(respData))
	}

	if len(respData) == 0 {
		return json.RawMessage("{}"), nil
	}
	return json.RawMessage(respData), nil
}

// executeSQL runs a SQL query via the Management API.
func (c *supabaseClient) executeSQL(ctx context.Context, query string) (json.RawMessage, error) {
	path := fmt.Sprintf("/v1/projects/%s/database/query", c.projectRef)
	return c.doJSON(ctx, http.MethodPost, path, map[string]string{"query": query})
}

// listTables queries information_schema for tables in the given schemas.
func (c *supabaseClient) listTables(ctx context.Context, schemas []string) (json.RawMessage, error) {
	if len(schemas) == 0 {
		schemas = []string{"public"}
	}
	// Build schema list for IN clause
	quoted := ""
	for i, s := range schemas {
		if i > 0 {
			quoted += ","
		}
		quoted += "'" + s + "'"
	}
	query := fmt.Sprintf(`SELECT table_schema, table_name, table_type FROM information_schema.tables WHERE table_schema IN (%s) ORDER BY table_schema, table_name`, quoted)
	return c.executeSQL(ctx, query)
}

// applyMigration tracks a DDL migration.
func (c *supabaseClient) applyMigration(ctx context.Context, name string, statements []string) error {
	path := fmt.Sprintf("/v1/projects/%s/database/migrations", c.projectRef)
	body := map[string]any{
		"name":       name,
		"statements": statements,
	}
	_, err := c.doJSON(ctx, http.MethodPost, path, body)
	return err
}

// listStorageBuckets returns all storage buckets.
func (c *supabaseClient) listStorageBuckets(ctx context.Context) (json.RawMessage, error) {
	path := fmt.Sprintf("/v1/projects/%s/storage/buckets", c.projectRef)
	return c.doJSON(ctx, http.MethodGet, path, nil)
}

// apiKey represents a Supabase API key.
type apiKey struct {
	Name   string `json:"name"`
	APIKey string `json:"api_key"`
}

// getAPIKeys returns all API keys for the project.
func (c *supabaseClient) getAPIKeys(ctx context.Context) ([]apiKey, error) {
	path := fmt.Sprintf("/v1/projects/%s/api-keys", c.projectRef)
	raw, err := c.doJSON(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}
	var keys []apiKey
	if err := json.Unmarshal(raw, &keys); err != nil {
		return nil, fmt.Errorf("parse API keys: %w", err)
	}
	return keys, nil
}

// getAuthConfig returns the current auth configuration.
func (c *supabaseClient) getAuthConfig(ctx context.Context) (json.RawMessage, error) {
	path := fmt.Sprintf("/v1/projects/%s/config/auth", c.projectRef)
	return c.doJSON(ctx, http.MethodGet, path, nil)
}

// updateAuthConfig patches the auth configuration.
func (c *supabaseClient) updateAuthConfig(ctx context.Context, config map[string]any) error {
	path := fmt.Sprintf("/v1/projects/%s/config/auth", c.projectRef)
	_, err := c.doJSON(ctx, http.MethodPatch, path, config)
	return err
}

// getLogs fetches recent logs for a service.
func (c *supabaseClient) getLogs(ctx context.Context, sql string) (json.RawMessage, error) {
	path := fmt.Sprintf("/v1/projects/%s/analytics/endpoints/logs.all?sql=%s", c.projectRef, sql)
	return c.doJSON(ctx, http.MethodGet, path, nil)
}

// setSecrets creates or updates edge function secrets.
// Body is a JSON array of {name, value} objects.
func (c *supabaseClient) setSecrets(ctx context.Context, secrets []map[string]string) error {
	path := fmt.Sprintf("/v1/projects/%s/secrets", c.projectRef)
	_, err := c.doJSON(ctx, http.MethodPost, path, secrets)
	return err
}

// listSecrets returns all project secrets.
func (c *supabaseClient) listSecrets(ctx context.Context) (json.RawMessage, error) {
	path := fmt.Sprintf("/v1/projects/%s/secrets", c.projectRef)
	return c.doJSON(ctx, http.MethodGet, path, nil)
}

// deleteSecrets removes secrets by name.
// Body is a JSON array of name strings.
func (c *supabaseClient) deleteSecrets(ctx context.Context, names []string) error {
	path := fmt.Sprintf("/v1/projects/%s/secrets", c.projectRef)
	_, err := c.doJSON(ctx, http.MethodDelete, path, names)
	return err
}
