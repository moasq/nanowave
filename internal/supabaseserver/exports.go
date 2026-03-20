package supabaseserver

import (
	"context"
	"encoding/json"
)

// Exported wrappers for CLI tool invocation.
// These assume env vars are already set (SUPABASE_ACCESS_TOKEN, SUPABASE_PROJECT_REF).

func ExecuteSQL(ctx context.Context, query string) (json.RawMessage, error) {
	c, err := newClientFromEnv()
	if err != nil {
		return nil, err
	}
	return c.executeSQL(ctx, query)
}

func ListTables(ctx context.Context, schemas []string) (json.RawMessage, error) {
	c, err := newClientFromEnv()
	if err != nil {
		return nil, err
	}
	return c.listTables(ctx, schemas)
}

func ApplyMigration(ctx context.Context, name string, statements []string) error {
	c, err := newClientFromEnv()
	if err != nil {
		return err
	}
	return c.applyMigration(ctx, name, statements)
}

func GetAnonKey(ctx context.Context) (string, error) {
	c, err := newClientFromEnv()
	if err != nil {
		return "", err
	}
	keys, err := c.getAPIKeys(ctx)
	if err != nil {
		return "", err
	}
	for _, k := range keys {
		if k.Name == "anon" || containsStr(k.Name, "anon") {
			return k.APIKey, nil
		}
	}
	return "", nil
}

func GetAuthConfig(ctx context.Context) (json.RawMessage, error) {
	c, err := newClientFromEnv()
	if err != nil {
		return nil, err
	}
	return c.getAuthConfig(ctx)
}

func ConfigureAuth(ctx context.Context, rawInput json.RawMessage) (json.RawMessage, error) {
	var input configureAuthInput
	if err := json.Unmarshal(rawInput, &input); err != nil {
		return nil, err
	}
	c, err := newClientFromEnv()
	if err != nil {
		return nil, err
	}

	config := make(map[string]any)
	var configured []string

	for _, p := range input.Providers {
		switch p.Name {
		case "email":
			configured = append(configured, "email")
			continue
		case "anonymous":
			config["EXTERNAL_ANONYMOUS_USERS_ENABLED"] = p.Enabled
			configured = append(configured, "anonymous")
			continue
		}
		mapper, ok := providerConfigMap[p.Name]
		if !ok {
			continue
		}
		for k, v := range mapper(p) {
			config[k] = v
		}
		configured = append(configured, p.Name)
	}

	if len(config) > 0 {
		if err := c.updateAuthConfig(ctx, config); err != nil {
			return nil, err
		}
	}

	result, _ := json.Marshal(map[string]any{"configured": configured})
	return result, nil
}

func containsStr(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
