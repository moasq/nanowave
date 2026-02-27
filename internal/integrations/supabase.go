package integrations

import (
	"bufio"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/zalando/go-keyring"
)

// supabaseProject represents a project from `supabase projects list --output json`.
type supabaseProject struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Region    string `json:"region"`
	CreatedAt string `json:"created_at"`
}

// supabaseOrg represents an organization from `supabase orgs list --output json`.
type supabaseOrg struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// supabaseAPIKey represents a key from `supabase projects api-keys --output json`.
// The CLI may use different field names across versions, so we try multiple.
type supabaseAPIKey struct {
	Name   string `json:"name"`
	APIKey string `json:"api_key"`
}

func (k *supabaseAPIKey) UnmarshalJSON(data []byte) error {
	// Try all known field name variants the CLI may use
	var raw map[string]interface{}
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	// Name field: "name", "key_name"
	for _, field := range []string{"name", "key_name"} {
		if v, ok := raw[field].(string); ok && v != "" {
			k.Name = v
			break
		}
	}
	// Key field: "api_key", "key"
	for _, field := range []string{"api_key", "key"} {
		if v, ok := raw[field].(string); ok && v != "" {
			k.APIKey = v
			break
		}
	}
	return nil
}

// supabaseRegions are the available Supabase project regions.
// Keep in sync with https://supabase.com/docs/guides/platform/regions
var supabaseRegions = []struct {
	ID    string
	Label string
}{
	{"us-east-1", "US East (N. Virginia)"},
	{"us-west-1", "US West (N. California)"},
	{"us-east-2", "US East (Ohio)"},
	{"us-west-2", "US West (Oregon)"},
	{"ca-central-1", "Canada (Central)"},
	{"eu-west-1", "EU West (Ireland)"},
	{"eu-west-2", "EU West (London)"},
	{"eu-west-3", "EU West (Paris)"},
	{"eu-central-1", "EU Central (Frankfurt)"},
	{"eu-central-2", "EU Central (Zurich)"},
	{"eu-north-1", "EU North (Stockholm)"},
	{"ap-southeast-1", "Asia Pacific (Singapore)"},
	{"ap-southeast-2", "Asia Pacific (Sydney)"},
	{"ap-northeast-1", "Asia Pacific (Tokyo)"},
	{"ap-northeast-2", "Asia Pacific (Seoul)"},
	{"ap-south-1", "Asia Pacific (Mumbai)"},
	{"sa-east-1", "South America (Sao Paulo)"},
}

// SetupSupabase runs the fully automatic Supabase credential flow:
// 1. supabase login (browser auth, skipped if already authenticated)
// 2. supabase orgs list (auto-select if one, picker if multiple)
// 3. supabase projects create (auto-create with appName)
// 4. supabase projects api-keys (extract anon key)
// 5. Validate + store
//
// appName is used as the Supabase project name. If empty, defaults to "my-app".
// printFn and pickFn are injected for testability.
func SetupSupabase(
	store *IntegrationStore,
	appName string,
	printFn func(level, msg string),
	pickFn func(title string, options []string) string,
) error {
	// Normalize app name for Supabase (lowercase, hyphens, no special chars)
	projectName := sanitizeProjectName(appName)

	// Step 1: Authenticate via Supabase CLI.
	// `supabase login` opens the browser for OAuth and stores the access token
	// in the system keychain (macOS) or ~/.supabase/access-token (Linux fallback).
	if IsSupabaseCLIAuthenticated() {
		printFn("success", "Supabase already authenticated")
	} else {
		printFn("info", "Opening browser to authenticate with Supabase...")
		loginCmd := exec.Command("supabase", "login")
		loginCmd.Stdin = os.Stdin
		loginCmd.Stdout = os.Stdout
		loginCmd.Stderr = os.Stderr
		if err := loginCmd.Run(); err != nil {
			return fmt.Errorf("supabase login failed: %w", err)
		}
		printFn("success", "Supabase authenticated")
	}

	// Step 1b: Read the access token that `supabase login` stored.
	// The token is needed for MCP tools and backend auto-provisioning.
	// Check keychain, env var, and file paths — then fall back to manual paste.
	pat := readSupabasePAT()
	if pat != "" {
		printFn("success", "Supabase access token found")
	} else {
		// Automatic detection failed — open browser and prompt as fallback
		printFn("warning", "Could not read Supabase access token automatically")
		printFn("info", "Opening token page — generate a new token and paste it below")
		pat = promptPAT()
		if pat != "" {
			saveSupabasePAT(pat)
			printFn("success", "Supabase access token saved")
		} else {
			printFn("warning", "No access token provided — backend auto-provisioning will not work")
			printFn("info", "You can add it later with: nanowave integrations setup supabase")
			return fmt.Errorf("supabase access token required for project setup")
		}
	}

	// Step 2: Select organization (auto if only one)
	orgID, err := resolveOrg(printFn, pickFn)
	if err != nil {
		return err
	}

	// Step 3: Pick region
	regionID, err := pickRegion(pickFn)
	if err != nil {
		return err
	}

	// Step 4: Create project (or reuse existing if limit reached)
	printFn("info", fmt.Sprintf("Creating project %q in %s...", projectName, regionID))

	dbPassword := generatePassword()
	createCmd := exec.Command("supabase", "projects", "create", projectName,
		"--org-id", orgID,
		"--region", regionID,
		"--db-password", dbPassword,
	)
	createOut, err := createCmd.CombinedOutput()

	var projectRef string
	if err != nil {
		createOutStr := strings.TrimSpace(string(createOut))
		// Detect free tier project limit error
		if strings.Contains(createOutStr, "maximum limits") || strings.Contains(createOutStr, "free projects") {
			printFn("warning", "Free project limit reached — let's reuse an existing project instead")
			ref, reuseErr := pickExistingProject(printFn, pickFn)
			if reuseErr != nil {
				return fmt.Errorf("failed to create project (limit reached) and could not reuse existing: %w", reuseErr)
			}
			projectRef = ref
		} else {
			return fmt.Errorf("failed to create project: %s", createOutStr)
		}
	} else {
		// Extract project ref from create output or list projects to find it
		ref, findErr := findCreatedProject(projectName, printFn)
		if findErr != nil {
			return findErr
		}
		projectRef = ref
	}

	// Step 5: Wait for project to be ready and fetch API keys
	printFn("info", "Waiting for project to initialize...")
	anonKey, err := waitForAPIKeys(projectRef, printFn)
	if err != nil {
		return err
	}

	// PAT was already verified in Step 1b above.

	projectURL := fmt.Sprintf("https://%s.supabase.co", projectRef)

	// Step 6: Validate
	if err := validateSupabaseConnection(projectURL, anonKey); err != nil {
		printFn("warning", fmt.Sprintf("Validation warning: %v (storing config anyway)", err))
	}

	cfg := IntegrationConfig{
		Provider:   ProviderSupabase,
		ProjectURL: projectURL,
		ProjectRef: projectRef,
		AnonKey:    anonKey,
		PAT:        pat,
	}

	if err := store.SetProvider(cfg, appName); err != nil {
		return fmt.Errorf("failed to save config: %w", err)
	}

	printFn("success", "Supabase connected")
	printFn("detail", fmt.Sprintf("Project: %s", projectName))
	printFn("detail", fmt.Sprintf("URL: %s", projectURL))

	return nil
}

// resolveOrg lists orgs and auto-selects if only one, otherwise shows a picker.
func resolveOrg(printFn func(level, msg string), pickFn func(string, []string) string) (string, error) {
	orgsCmd := exec.Command("supabase", "orgs", "list", "--output", "json")
	orgsOut, err := orgsCmd.Output()
	if err != nil {
		return "", fmt.Errorf("failed to list organizations: %w", err)
	}

	var orgs []supabaseOrg
	if err := json.Unmarshal(orgsOut, &orgs); err != nil {
		return "", fmt.Errorf("failed to parse organizations: %w", err)
	}

	if len(orgs) == 0 {
		return "", fmt.Errorf("no Supabase organizations found — create one at supabase.com/dashboard")
	}

	// Auto-select if only one org
	if len(orgs) == 1 {
		printFn("detail", fmt.Sprintf("Organization: %s", orgs[0].Name))
		return orgs[0].ID, nil
	}

	// Multiple orgs — let user pick
	options := make([]string, len(orgs))
	for i, o := range orgs {
		options[i] = o.Name
	}

	picked := pickFn("Select organization", options)
	if picked == "" {
		return "", fmt.Errorf("no organization selected")
	}

	for _, o := range orgs {
		if o.Name == picked {
			return o.ID, nil
		}
	}
	return "", fmt.Errorf("could not match selected organization")
}

// pickRegion shows a region picker and returns the selected region ID.
func pickRegion(pickFn func(string, []string) string) (string, error) {
	options := make([]string, len(supabaseRegions))
	for i, r := range supabaseRegions {
		options[i] = fmt.Sprintf("%s — %s", r.Label, r.ID)
	}

	picked := pickFn("Select region", options)
	if picked == "" {
		return "", fmt.Errorf("no region selected")
	}

	// Extract region ID from "Label — id" format
	for _, r := range supabaseRegions {
		label := fmt.Sprintf("%s — %s", r.Label, r.ID)
		if label == picked {
			return r.ID, nil
		}
	}
	return "", fmt.Errorf("could not match selected region")
}

// pickExistingProject lists the user's Supabase projects and lets them pick one to reuse.
// Used when the free project limit is reached and a new project cannot be created.
func pickExistingProject(printFn func(level, msg string), pickFn func(string, []string) string) (string, error) {
	listCmd := exec.Command("supabase", "projects", "list", "--output", "json")
	listOut, err := listCmd.Output()
	if err != nil {
		return "", fmt.Errorf("failed to list projects: %w", err)
	}

	var projects []supabaseProject
	if err := json.Unmarshal(listOut, &projects); err != nil {
		return "", fmt.Errorf("failed to parse projects: %w", err)
	}

	if len(projects) == 0 {
		return "", fmt.Errorf("no existing projects found — delete a project at supabase.com/dashboard or upgrade your plan")
	}

	options := make([]string, len(projects))
	for i, p := range projects {
		options[i] = fmt.Sprintf("%s (%s)", p.Name, p.ID)
	}

	picked := pickFn("Select an existing project to reuse", options)
	if picked == "" {
		return "", fmt.Errorf("no project selected")
	}

	// Extract project ref from "name (ref)" format
	for _, p := range projects {
		label := fmt.Sprintf("%s (%s)", p.Name, p.ID)
		if label == picked {
			printFn("success", fmt.Sprintf("Reusing project: %s", p.Name))
			return p.ID, nil
		}
	}
	return "", fmt.Errorf("could not match selected project")
}

// findCreatedProject lists projects and finds the one matching projectName.
// Newly created projects appear at the top of the list.
func findCreatedProject(projectName string, printFn func(level, msg string)) (string, error) {
	// Give the API a moment to register the project
	time.Sleep(2 * time.Second)

	listCmd := exec.Command("supabase", "projects", "list", "--output", "json")
	listOut, err := listCmd.Output()
	if err != nil {
		return "", fmt.Errorf("failed to list projects after creation: %w", err)
	}

	var projects []supabaseProject
	if err := json.Unmarshal(listOut, &projects); err != nil {
		return "", fmt.Errorf("failed to parse projects: %w", err)
	}

	// Find by exact name match (newest first — Supabase returns newest first)
	for _, p := range projects {
		if p.Name == projectName {
			return p.ID, nil
		}
	}

	// Fallback: return the first project (most recently created)
	if len(projects) > 0 {
		printFn("warning", fmt.Sprintf("Could not match project %q by name, using most recent: %s", projectName, projects[0].Name))
		return projects[0].ID, nil
	}

	return "", fmt.Errorf("created project not found in project list")
}

// waitForAPIKeys polls for API keys until they're available (project initialization).
// New projects may take 30-60 seconds before API keys are ready.
func waitForAPIKeys(projectRef string, printFn func(level, msg string)) (string, error) {
	maxAttempts := 20
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		anonKey, err := fetchAnonKey(projectRef)
		if err == nil && anonKey != "" {
			return anonKey, nil
		}

		if attempt < maxAttempts {
			time.Sleep(3 * time.Second)
		}
	}

	return "", fmt.Errorf("timed out waiting for project API keys (project may still be initializing — try again in a minute)")
}

// fetchAnonKey retrieves the anon key for a project.
func fetchAnonKey(projectRef string) (string, error) {
	keysCmd := exec.Command("supabase", "projects", "api-keys", "--project-ref", projectRef, "--output", "json")
	keysOut, err := keysCmd.Output()
	if err != nil {
		return "", err
	}

	var keys []supabaseAPIKey
	if err := json.Unmarshal(keysOut, &keys); err != nil {
		return "", err
	}

	// Try matching by name (case-insensitive)
	for _, k := range keys {
		nameLower := strings.ToLower(k.Name)
		if nameLower == "anon" || strings.Contains(nameLower, "anon") {
			return k.APIKey, nil
		}
	}

	// Fallback: first non-service key
	for _, k := range keys {
		nameLower := strings.ToLower(k.Name)
		if !strings.Contains(nameLower, "service") && k.APIKey != "" {
			return k.APIKey, nil
		}
	}

	// Last resort: if there are exactly 2 keys, one is anon and one is service_role.
	// The anon key is typically the shorter JWT or the first one listed.
	if len(keys) == 2 {
		for _, k := range keys {
			if k.APIKey != "" {
				return k.APIKey, nil
			}
		}
	}

	return "", fmt.Errorf("anon key not found")
}

// sanitizeProjectName converts an app name to a valid Supabase project name.
// Supabase project names: lowercase, hyphens allowed, no special characters.
func sanitizeProjectName(name string) string {
	if name == "" {
		return "my-app"
	}
	// Convert PascalCase/camelCase to kebab-case
	var result []rune
	for i, r := range name {
		if r >= 'A' && r <= 'Z' {
			if i > 0 {
				result = append(result, '-')
			}
			result = append(result, r+32) // toLower
		} else if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			result = append(result, r)
		} else if r == ' ' || r == '_' {
			result = append(result, '-')
		}
		// Skip other characters
	}
	s := string(result)
	s = strings.Trim(s, "-")
	if s == "" {
		return "my-app"
	}
	return s
}

// generatePassword creates a random 24-character hex password for the database.
func generatePassword() string {
	b := make([]byte, 12)
	if _, err := rand.Read(b); err != nil {
		// Fallback to a static password if crypto/rand fails (shouldn't happen)
		return "nanowave-db-password-2024"
	}
	return hex.EncodeToString(b)
}

// SetupSupabaseManual lets the user enter Supabase credentials directly.
// appName scopes the credentials to a specific app project.
// readLineFn prompts for and reads a single line of input.
func SetupSupabaseManual(
	store *IntegrationStore,
	appName string,
	readLineFn func(label string) string,
	printFn func(level, msg string),
) error {
	printFn("info", "Enter your Supabase project credentials (from supabase.com/dashboard → Settings → API)")

	projectURL := readLineFn("Project URL (e.g. https://xyz.supabase.co)")
	if projectURL == "" {
		return fmt.Errorf("project URL is required")
	}
	projectURL = strings.TrimRight(projectURL, "/")

	anonKey := readLineFn("Anon Key")
	if anonKey == "" {
		return fmt.Errorf("anon key is required")
	}

	// Extract project ref from URL: https://xyz.supabase.co → xyz
	projectRef := ""
	if strings.Contains(projectURL, ".supabase.co") {
		host := strings.TrimPrefix(projectURL, "https://")
		host = strings.TrimPrefix(host, "http://")
		if idx := strings.Index(host, "."); idx > 0 {
			projectRef = host[:idx]
		}
	}

	// Validate connection
	if err := validateSupabaseConnection(projectURL, anonKey); err != nil {
		printFn("warning", fmt.Sprintf("Validation warning: %v (storing config anyway)", err))
	}

	cfg := IntegrationConfig{
		Provider:   ProviderSupabase,
		ProjectURL: projectURL,
		ProjectRef: projectRef,
		AnonKey:    anonKey,
	}

	if err := store.SetProvider(cfg, appName); err != nil {
		return fmt.Errorf("failed to save config: %w", err)
	}

	printFn("success", "Supabase connected")
	printFn("detail", fmt.Sprintf("URL: %s", projectURL))
	if projectRef != "" {
		printFn("detail", fmt.Sprintf("Project ref: %s", projectRef))
	}

	return nil
}

// IsSupabaseCLIAuthenticated checks if the Supabase CLI is already logged in
// by trying to list projects. If it succeeds, we're authenticated.
func IsSupabaseCLIAuthenticated() bool {
	cmd := exec.Command("supabase", "projects", "list", "--output", "json")
	err := cmd.Run()
	return err == nil
}

// accessTokenPattern matches valid Supabase PATs: sbp_[0-9a-f]{40} or sbp_oauth_[0-9a-f]{40}.
// This is the same regex the Supabase CLI uses for validation.
var accessTokenPattern = regexp.MustCompile(`^sbp_(oauth_)?[a-f0-9]{40}$`)

// isValidPAT checks whether a token matches the Supabase PAT format.
func isValidPAT(token string) bool {
	return accessTokenPattern.MatchString(token)
}

// readSupabasePAT reads the Supabase PAT from multiple sources in priority order:
// 1. SUPABASE_ACCESS_TOKEN env var (CI/CD override)
// 2. OS-native credential store via go-keyring (nanowave's own cached copy)
// 3. Supabase CLI's keyring entries (profile "supabase" or legacy "access-token")
// 4. ~/.supabase/access-token file (Supabase CLI file fallback)
// 5. ~/.config/supabase/access-token file (XDG fallback)
// All tokens are validated against the Supabase PAT format before being returned.
func readSupabasePAT() string {
	// 1. Env var takes priority (for CI/CD environments)
	if token := strings.TrimSpace(os.Getenv("SUPABASE_ACCESS_TOKEN")); token != "" && isValidPAT(token) {
		return token
	}

	// 2. Our own cached token in OS-native credential store
	if token := readFromKeychain("nanowave", "supabase-pat"); token != "" && isValidPAT(token) {
		return token
	}

	// 3. Supabase CLI's keyring entries (same library, same service name).
	// Current CLI uses profile name "supabase" as account; legacy used "access-token".
	for _, account := range []string{"supabase", "access-token"} {
		if token := readFromKeychain("Supabase CLI", account); token != "" && isValidPAT(token) {
			saveSupabasePAT(token) // Cache in our own keyring entry for reliability
			return token
		}
	}

	// 4. File-based fallbacks (Supabase CLI writes here when keyring is unavailable)
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	for _, path := range []string{
		filepath.Join(home, ".supabase", "access-token"),
		filepath.Join(home, ".config", "supabase", "access-token"),
	} {
		if data, err := os.ReadFile(path); err == nil {
			if token := strings.TrimSpace(string(data)); token != "" && isValidPAT(token) {
				saveSupabasePAT(token) // Cache in keyring
				return token
			}
		}
	}

	return ""
}

// readFromKeychain reads a password from the system keychain using zalando/go-keyring.
// This is the same library the Supabase CLI uses to store tokens, ensuring compatible
// access and automatic handling of all encoding formats (base64, hex).
func readFromKeychain(service, account string) string {
	token, err := keyring.Get(service, account)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(token)
}

// supabaseTokensURL is the dashboard page where users generate access tokens.
const supabaseTokensURL = "https://supabase.com/dashboard/account/tokens"

// promptPAT opens the Supabase tokens page in the browser and prompts the user to paste their access token.
// Returns an empty string if the user skips or provides an invalid token.
func promptPAT() string {
	// Open the tokens page in the default browser
	_ = exec.Command("open", supabaseTokensURL).Start()
	fmt.Print("  Paste your access token (or press Enter to skip): ")
	reader := bufio.NewReader(os.Stdin)
	line, _ := reader.ReadString('\n')
	token := strings.TrimSpace(line)
	if token == "" {
		return ""
	}
	if !isValidPAT(token) {
		fmt.Println("  Invalid token format — expected sbp_xxxx (40 hex chars)")
		return ""
	}
	return token
}

// saveSupabasePAT persists the PAT in the OS-native credential store (keyring).
// Falls back to ~/.nanowave/supabase-pat file if keyring is unavailable.
// Always cleans up the legacy plain-text file when keyring succeeds.
func saveSupabasePAT(pat string) {
	// Try OS-native credential store first (secure)
	if err := keyring.Set("nanowave", "supabase-pat", pat); err == nil {
		// Clean up legacy plain-text file now that keyring works
		removeLegacyPATFile()
		return
	}
	// Fallback to file (e.g. Linux without D-Bus / Secret Service)
	home, err := os.UserHomeDir()
	if err != nil {
		return
	}
	dir := filepath.Join(home, ".nanowave")
	os.MkdirAll(dir, 0700)
	os.WriteFile(filepath.Join(dir, "supabase-pat"), []byte(pat), 0600)
}

// removeLegacyPATFile deletes the old plain-text ~/.nanowave/supabase-pat file
// left over from previous versions or failed keyring saves.
func removeLegacyPATFile() {
	home, err := os.UserHomeDir()
	if err != nil {
		return
	}
	os.Remove(filepath.Join(home, ".nanowave", "supabase-pat"))
}

// validateSupabaseConnection tests that the project URL and anon key are valid.
func validateSupabaseConnection(projectURL, anonKey string) error {
	client := &http.Client{Timeout: 10 * time.Second}
	req, err := http.NewRequest("GET", projectURL+"/rest/v1/", nil)
	if err != nil {
		return err
	}
	req.Header.Set("apikey", anonKey)
	req.Header.Set("Authorization", "Bearer "+anonKey)

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("connection failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return fmt.Errorf("API returned status %d", resp.StatusCode)
	}
	return nil
}

// RevokeSupabase fully removes all Supabase credentials and config for an app:
// keyring entries, cached PAT file, and the integration config from the store.
func RevokeSupabase(store *IntegrationStore, appName string) error {
	// Remove our cached PAT from OS keyring
	_ = keyring.Delete("nanowave", "supabase-pat")

	// Remove cached PAT file
	if home, err := os.UserHomeDir(); err == nil {
		os.Remove(filepath.Join(home, ".nanowave", "supabase-pat"))
	}

	// Remove integration config from store
	return store.RemoveProvider(ProviderSupabase, appName)
}
