package revenuecat

import (
	"context"
	"fmt"
	"os/exec"
	"strings"

	"github.com/moasq/nanowave/internal/integrations"
)

const (
	rcDashboardURL = "https://app.revenuecat.com"
)

// Setup runs the interactive setup flow for RevenueCat.
func (r *revenuecatProvider) Setup(_ context.Context, req integrations.SetupRequest) error {
	if req.Manual {
		return r.setupManual(req)
	}
	return r.setupGuided(req)
}

func (r *revenuecatProvider) setupGuided(req integrations.SetupRequest) error {
	req.PrintFn("header", "RevenueCat Setup (guided)")
	req.PrintFn("info", "")
	req.PrintFn("info", "We need a v2 secret API key (starts with sk_) from your RevenueCat dashboard.")
	req.PrintFn("info", "")
	req.PrintFn("detail", "1. Open your project in the RevenueCat dashboard")
	req.PrintFn("detail", "2. Click 'API Keys' in the left sidebar")
	req.PrintFn("detail", "3. Scroll to 'Secret API keys' and click '+ New secret API key'")
	req.PrintFn("detail", "4. Name it (e.g. 'nanowave'), select version 'V2', enable write access")
	req.PrintFn("detail", "5. Click 'Generate' and copy the key (starts with sk_)")
	req.PrintFn("info", "")

	// Let the user read the instructions, then open the dashboard on Enter.
	// If the user already has the key, they can paste it directly here.
	input := strings.TrimSpace(req.ReadLineFn("▶ Press Enter to open RevenueCat dashboard, or paste your sk_ key now"))

	var secretKey string
	if strings.HasPrefix(input, "sk_") {
		// User pasted the key directly — skip opening the browser
		secretKey = input
	} else {
		// Open the dashboard and wait for the key
		_ = exec.Command("open", rcDashboardURL).Start()
		req.PrintFn("info", "Dashboard opened — follow the steps above, then paste the key here")
		secretKey = strings.TrimSpace(req.ReadLineFn("Paste your secret API key (sk_...)"))
	}
	if secretKey == "" {
		return fmt.Errorf("no key provided — run setup again when ready")
	}
	if !strings.HasPrefix(secretKey, "sk_") {
		return fmt.Errorf("invalid key — must start with sk_ (you pasted %q)", truncateKey(secretKey))
	}

	client := newRCClient(secretKey)
	ctx := context.Background()

	// Validate the key immediately
	req.PrintFn("info", "Validating key...")
	projects, err := client.listProjects(ctx)
	if err != nil {
		return fmt.Errorf("invalid key or API error: %w", err)
	}
	if len(projects) == 0 {
		return fmt.Errorf("no projects found — create a project at %s first", rcDashboardURL)
	}
	req.PrintFn("success", "Key valid — found your account")

	// Auto-select project if only one, otherwise let user pick
	var selectedProject rcProject
	if len(projects) == 1 {
		selectedProject = projects[0]
		req.PrintFn("success", fmt.Sprintf("Project: %s", selectedProject.Name))
	} else {
		var projectOptions []string
		for _, p := range projects {
			projectOptions = append(projectOptions, fmt.Sprintf("%s (%s)", p.Name, p.ID))
		}
		picked := req.PickFn("Select a project", projectOptions)
		for _, p := range projects {
			if strings.Contains(picked, p.ID) {
				selectedProject = p
				break
			}
		}
		if selectedProject.ID == "" {
			return fmt.Errorf("no project selected")
		}
	}

	// List apps and pick or create
	apps, err := client.listApps(ctx, selectedProject.ID)
	if err != nil {
		return fmt.Errorf("failed to list apps: %w", err)
	}

	var selectedApp rcApp
	if len(apps) == 0 {
		req.PrintFn("info", "No iOS apps found in this project — let's create one")
		selectedApp, err = r.createNewApp(ctx, client, selectedProject.ID, req)
		if err != nil {
			return err
		}
	} else if len(apps) == 1 {
		selectedApp = apps[0]
		req.PrintFn("success", fmt.Sprintf("App: %s (%s)", selectedApp.Name, selectedApp.BundleID()))
	} else {
		var appOptions []string
		for _, a := range apps {
			label := a.Name
			if a.BundleID() != "" {
				label = fmt.Sprintf("%s — %s", a.Name, a.BundleID())
			}
			appOptions = append(appOptions, label)
		}
		appOptions = append(appOptions, "Create new app")
		appPicked := req.PickFn("Select an app", appOptions)
		if appPicked == "Create new app" {
			selectedApp, err = r.createNewApp(ctx, client, selectedProject.ID, req)
			if err != nil {
				return err
			}
		} else {
			for _, a := range apps {
				if strings.Contains(appPicked, a.Name) {
					selectedApp = a
					break
				}
			}
		}
	}
	if selectedApp.ID == "" {
		return fmt.Errorf("no app selected")
	}

	// Auto-retrieve public API key
	publicKey := ""
	keys, err := client.getPublicAPIKeys(ctx, selectedProject.ID, selectedApp.ID)
	if err == nil {
		for _, k := range keys {
			if strings.HasPrefix(k.Key, "appl_") {
				publicKey = k.Key
				break
			}
		}
		if publicKey == "" {
			for _, k := range keys {
				if strings.HasPrefix(k.Key, "test_") {
					publicKey = k.Key
					break
				}
			}
		}
		if publicKey == "" && len(keys) > 0 {
			publicKey = keys[0].Key
		}
	}

	if publicKey != "" {
		req.PrintFn("success", fmt.Sprintf("Public SDK key: %s...%s", publicKey[:8], publicKey[len(publicKey)-4:]))
	} else {
		req.PrintFn("warning", "No public SDK key found — the build will use a placeholder")
	}

	// Store config
	cfg := integrations.IntegrationConfig{
		Provider:   integrations.ProviderRevenueCat,
		ProjectURL: selectedProject.ID,
		ProjectRef: selectedApp.ID,
		AnonKey:    publicKey,
		PAT:        secretKey,
	}
	if err := req.Store.SetProvider(cfg, req.AppName); err != nil {
		return fmt.Errorf("failed to save config: %w", err)
	}

	req.PrintFn("success", "RevenueCat configured — everything else is automatic")
	return nil
}

func (r *revenuecatProvider) createNewApp(ctx context.Context, client *rcClient, projectID string, req integrations.SetupRequest) (rcApp, error) {
	name := strings.TrimSpace(req.ReadLineFn("App name"))
	if name == "" {
		return rcApp{}, fmt.Errorf("app name is required")
	}
	bundleID := strings.TrimSpace(req.ReadLineFn("Bundle ID (e.g. com.yourcompany.appname)"))
	if bundleID == "" {
		return rcApp{}, fmt.Errorf("bundle ID is required")
	}
	req.PrintFn("info", "Creating app...")
	app, err := client.createApp(ctx, projectID, name, bundleID)
	if err != nil {
		return rcApp{}, fmt.Errorf("failed to create app: %w", err)
	}
	req.PrintFn("success", fmt.Sprintf("Created app: %s", app.Name))
	return *app, nil
}

func (r *revenuecatProvider) setupManual(req integrations.SetupRequest) error {
	req.PrintFn("header", "RevenueCat Manual Setup")
	req.PrintFn("detail", "Enter your credentials from https://app.revenuecat.com")

	projectID := strings.TrimSpace(req.ReadLineFn("Project ID (e.g. proj1a2b3c4d)"))
	if projectID == "" {
		return fmt.Errorf("project ID is required")
	}

	appID := strings.TrimSpace(req.ReadLineFn("App ID (e.g. app1a2b3c4)"))
	if appID == "" {
		return fmt.Errorf("app ID is required")
	}

	secretKey := strings.TrimSpace(req.ReadLineFn("Secret API key (sk_...)"))
	if secretKey == "" {
		return fmt.Errorf("secret API key is required")
	}

	publicKey := strings.TrimSpace(req.ReadLineFn("Public API key (appl_... or test_..., Enter to skip)"))

	// Validate connection
	req.PrintFn("info", "Validating...")
	client := newRCClient(secretKey)
	if err := client.validateConnection(context.Background(), projectID); err != nil {
		return fmt.Errorf("validation failed: %w", err)
	}

	cfg := integrations.IntegrationConfig{
		Provider:   integrations.ProviderRevenueCat,
		ProjectURL: projectID,
		ProjectRef: appID,
		AnonKey:    publicKey,
		PAT:        secretKey,
	}
	if err := req.Store.SetProvider(cfg, req.AppName); err != nil {
		return fmt.Errorf("failed to save config: %w", err)
	}
	req.PrintFn("success", "RevenueCat configured")
	return nil
}

// Remove removes the RevenueCat config for an app.
func (r *revenuecatProvider) Remove(_ context.Context, store *integrations.IntegrationStore, appName string) error {
	return store.RemoveProvider(integrations.ProviderRevenueCat, appName)
}

// Status returns the current integration status for an app.
func (r *revenuecatProvider) Status(_ context.Context, store *integrations.IntegrationStore, appName string) (integrations.ProviderStatus, error) {
	cfg, err := store.GetProvider(integrations.ProviderRevenueCat, appName)
	if err != nil {
		return integrations.ProviderStatus{}, err
	}
	if cfg == nil {
		return integrations.ProviderStatus{Configured: false}, nil
	}
	return integrations.ProviderStatus{
		Configured: true,
		ProjectURL: cfg.ProjectURL,
		HasAnonKey: cfg.AnonKey != "",
		HasPAT:     cfg.PAT != "",
	}, nil
}

// CLIAvailable returns false — RevenueCat has no CLI tool.
// Setup always requires user input (API key prompt), so the pipeline
// routes through promptManualSetup which provides ReadLineFn.
func (r *revenuecatProvider) CLIAvailable() bool {
	return false
}

// truncateKey shows first 4 and last 4 chars of a key for error messages.
func truncateKey(key string) string {
	if len(key) <= 8 {
		return key
	}
	return key[:4] + "..." + key[len(key)-4:]
}
