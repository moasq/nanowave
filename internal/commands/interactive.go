package commands

import (
	"context"
	"crypto/sha256"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/moasq/nanowave/internal/config"
	"github.com/moasq/nanowave/internal/service"
	"github.com/moasq/nanowave/internal/storage"
	"github.com/moasq/nanowave/internal/terminal"
	"github.com/moasq/nanowave/internal/update"
	"github.com/spf13/cobra"
)

var chatCmd = &cobra.Command{
	Use:   "chat",
	Short: "Interactive chat mode",
	Long:  "Start an interactive session to build and edit your app through conversation.",
	RunE: func(cmd *cobra.Command, args []string) error {
		return runInteractive(cmd)
	},
}

// imageCache manages cached image files for the session.
type imageCache struct {
	dir    string            // temp directory for cached images
	cached map[string]string // original path → cached path
}

// cancelHolder safely shares the active operation cancel func across goroutines.
type cancelHolder struct {
	mu sync.Mutex
	fn context.CancelFunc
}

func (h *cancelHolder) Set(fn context.CancelFunc) {
	h.mu.Lock()
	h.fn = fn
	h.mu.Unlock()
}

// Take returns and clears the current cancel func atomically.
func (h *cancelHolder) Take() context.CancelFunc {
	h.mu.Lock()
	defer h.mu.Unlock()
	fn := h.fn
	h.fn = nil
	return fn
}

func (h *cancelHolder) Clear() {
	h.mu.Lock()
	h.fn = nil
	h.mu.Unlock()
}

func newImageCache() (*imageCache, error) {
	dir, err := os.MkdirTemp("", "nanowave-images-*")
	if err != nil {
		return nil, err
	}
	return &imageCache{dir: dir, cached: make(map[string]string)}, nil
}

// add copies an image to the cache and returns the cached path.
// If already cached, returns the existing cached path.
func (ic *imageCache) add(imagePath string) (string, error) {
	absPath, err := filepath.Abs(imagePath)
	if err != nil {
		absPath = imagePath
	}

	if cached, ok := ic.cached[absPath]; ok {
		if _, err := os.Stat(cached); err == nil {
			return cached, nil
		}
		// Cached file was deleted, re-cache
		delete(ic.cached, absPath)
	}

	src, err := os.Open(absPath)
	if err != nil {
		return "", fmt.Errorf("cannot read image %s: %w", absPath, err)
	}
	defer src.Close()

	// Use hash of original path + extension for the cached filename
	hash := sha256.Sum256([]byte(absPath))
	ext := filepath.Ext(absPath)
	cachedName := fmt.Sprintf("%x%s", hash[:8], ext)
	cachedPath := filepath.Join(ic.dir, cachedName)

	dst, err := os.Create(cachedPath)
	if err != nil {
		return "", fmt.Errorf("cannot create cached image: %w", err)
	}
	defer dst.Close()

	if _, err := io.Copy(dst, src); err != nil {
		os.Remove(cachedPath)
		return "", fmt.Errorf("cannot copy image: %w", err)
	}

	ic.cached[absPath] = cachedPath
	return cachedPath, nil
}

// addAll copies multiple images and returns their cached paths.
func (ic *imageCache) addAll(images []string) []string {
	var cached []string
	for _, img := range images {
		if path, err := ic.add(img); err == nil {
			cached = append(cached, path)
		}
	}
	return cached
}

// cleanup removes the cache directory and all cached images.
func (ic *imageCache) cleanup() {
	if ic.dir != "" {
		os.RemoveAll(ic.dir)
	}
	ic.cached = nil
}

// clear removes all cached images but keeps the directory.
func (ic *imageCache) clear() {
	entries, err := os.ReadDir(ic.dir)
	if err != nil {
		return
	}
	for _, entry := range entries {
		os.Remove(filepath.Join(ic.dir, entry.Name()))
	}
	ic.cached = make(map[string]string)
}

func runInteractive(cmd *cobra.Command) error {
	// Print welcome banner first (before config load which may fail)
	terminal.Banner(Version)

	// Check for updates in the background (non-blocking)
	updateCh := make(chan *update.Result, 1)
	go func() {
		updateCh <- update.Check("moasq", "nanowave", Version)
	}()

	// Print tool status
	var claudeVersion string
	var claudePath string
	if config.CheckClaude() {
		claudePath, _ = exec.LookPath("claude")
		if claudePath != "" {
			claudeVersion = config.ClaudeVersion(claudePath)
		}
	}

	// Check auth status
	var authStatus *config.ClaudeAuthStatus
	if claudePath != "" {
		authStatus = config.CheckClaudeAuth(claudePath)
	}

	toolOpts := terminal.ToolStatusOpts{
		ClaudeVersion: claudeVersion,
		HasXcode:      config.CheckXcode(),
		HasXcodeCLT:   config.CheckXcodeCLT(),
		HasSimulator:  config.CheckSimulator(),
		HasXcodegen:   config.CheckXcodegen(),
	}
	if authStatus != nil {
		toolOpts.AuthLoggedIn = authStatus.LoggedIn
		toolOpts.AuthEmail = authStatus.Email
		toolOpts.AuthPlan = authStatus.SubscriptionType
	}
	terminal.ToolStatus(toolOpts)

	// Show update warning if a newer version is available
	select {
	case res := <-updateCh:
		if res.NeedsUpdate() {
			terminal.Warning(fmt.Sprintf("Update available: v%s → v%s", res.Current, res.Latest))
			fmt.Println()
		}
	case <-time.After(3 * time.Second):
		// Don't block startup if the check is slow
	}

	// Auto-run setup on first launch if critical dependencies are missing
	if needsSetup() {
		if err := runSetup(); err != nil {
			return err
		}
		fmt.Println()
		// Re-check after setup — if still missing, exit
		if needsSetup() {
			terminal.Error("Some dependencies are still missing. Please install them and try again.")
			return fmt.Errorf("setup incomplete")
		}
	}

	cfg, err := config.Load()
	if err != nil {
		terminal.Error("Claude Code CLI is not installed.")
		terminal.Info("Run `nanowave setup` to install all prerequisites.")
		return err
	}

	// Project selection flow
	projects := cfg.ListProjects()
	if len(projects) > 0 {
		selected := showProjectPicker(projects)
		if selected == nil {
			// User picked "New project" — stay in build mode (ProjectDir = catalog root)
			fmt.Printf("  %sDescribe the app you want to build.%s\n", terminal.Dim, terminal.Reset)
			fmt.Println()
		} else {
			cfg.SetProject(selected.Path)
		}
	} else {
		fmt.Printf("  %sNo projects yet. Describe the app you want to build.%s\n", terminal.Dim, terminal.Reset)
		fmt.Println()
	}

	svc, err := service.NewService(cfg, service.ServiceOpts{Model: ModelFlag()})
	if err != nil {
		return err
	}

	// Initialize image cache
	imgCache, err := newImageCache()
	if err != nil {
		terminal.Warning(fmt.Sprintf("Image support unavailable: %v", err))
		imgCache = nil
	}
	if imgCache != nil {
		defer imgCache.cleanup()
	}

	// Print project status if a project is selected
	if cfg.HasProject() {
		svc.Info()
		fmt.Println()
	}

	fmt.Printf("  %sPress Enter to submit. Esc+Enter for newline. Drag images to attach.%s\n\n", terminal.Dim, terminal.Reset)

	// Set up signal handling for Ctrl+C
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Track whether an operation is in progress
	var activeCancel cancelHolder

	// Handle signals in background
	go func() {
		for range sigChan {
			if cancel := activeCancel.Take(); cancel != nil {
				// Cancel the running operation
				cancel()
				fmt.Println()
				terminal.Warning("Operation cancelled.")
				fmt.Println()
				terminal.Prompt()
			} else {
				// No operation running — exit
				fmt.Println()
				terminal.Info("Goodbye!")
				if imgCache != nil {
					imgCache.cleanup()
				}
				os.Exit(0)
			}
		}
	}()

	for {
		result := terminal.ReadInput()
		input := result.Text
		if input == "" && len(result.Images) == 0 {
			continue
		}

		// Handle slash commands
		if strings.HasPrefix(input, "/") && len(result.Images) == 0 {
			if input == "/clear" && imgCache != nil {
				imgCache.clear()
			}
			// Handle /projects specially since it needs to modify cfg/svc
			if input == "/projects" {
				projects := cfg.ListProjects()
				if len(projects) == 0 {
					terminal.Info("No projects yet. Describe the app you want to build.")
					fmt.Println()
				} else {
					selected := showProjectPicker(projects)
					if selected == nil {
						// New project
						newCfg, _ := config.Load()
						if newCfg != nil {
							cfg = newCfg
							svc.UpdateConfig(cfg)
						}
						fmt.Printf("  %sDescribe the app you want to build.%s\n", terminal.Dim, terminal.Reset)
						fmt.Println()
					} else {
						newCfg, _ := config.Load()
						if newCfg != nil {
							newCfg.SetProject(selected.Path)
							cfg = newCfg
							svc.UpdateConfig(cfg)
						}
						svc.Info()
						fmt.Println()
					}
				}
				continue
			}
			handled := handleSlashCommand(input, cfg, svc, cmd, authStatus)
			if handled {
				continue
			}
		}

		// Handle quit/exit text
		if input == "quit" || input == "exit" {
			terminal.Info("Goodbye!")
			break
		}

		// Handle image-only input (no text)
		if input == "" && len(result.Images) > 0 {
			for _, img := range result.Images {
				terminal.Detail("Image attached", filepath.Base(img))
			}
			terminal.Info("Attached image(s). Type your prompt and press Enter.")
			continue
		}

		// Check auth before sending
		if authStatus == nil || !authStatus.LoggedIn {
			terminal.Warning("Not signed in to Claude. Run `claude auth login` to authenticate.")
			fmt.Println()
			continue
		}

		// Cache any attached images
		var cachedImages []string
		if imgCache != nil && len(result.Images) > 0 {
			cachedImages = imgCache.addAll(result.Images)
			for _, img := range result.Images {
				terminal.Detail("Image", filepath.Base(img))
			}
		}

		// Create cancellable context for this operation
		ctx, cancel := context.WithCancel(cmd.Context())
		activeCancel.Set(cancel)

		// Capture usage before operation for delta
		usageBefore := svc.Usage()

		// Unified send — auto-routes build vs edit
		if err := svc.Send(ctx, input, cachedImages); err != nil {
			if ctx.Err() == nil {
				terminal.Error(fmt.Sprintf("Failed: %v", err))
			}
		} else {
			// Show post-operation cost summary
			usageAfter := svc.Usage()
			if usageAfter.Requests > usageBefore.Requests {
				costDelta := usageAfter.TotalCostUSD - usageBefore.TotalCostUSD
				tokenDelta := (usageAfter.InputTokens + usageAfter.OutputTokens) - (usageBefore.InputTokens + usageBefore.OutputTokens)
				if costDelta > 0 || tokenDelta > 0 {
					fmt.Printf("  %s$%.2f  ·  %s tokens%s\n",
						terminal.Dim, costDelta, storage.FormatTokenCount(tokenDelta), terminal.Reset)
				}
			}
		}

		// Clean up cached images after each request — they're single-use
		if imgCache != nil && len(cachedImages) > 0 {
			imgCache.clear()
		}

		// Reload config after potential build (config now has project)
		newCfg, _ := config.Load()
		if newCfg != nil {
			// If we had a project selected, keep it selected
			if cfg.NanowaveDir != "" {
				newCfg.SetProject(cfg.ProjectDir)
			} else {
				// After a build, check if a new project appeared and select it
				newProjects := newCfg.ListProjects()
				if len(newProjects) > 0 {
					// Select the most recent project (first in sorted list)
					newCfg.SetProject(newProjects[0].Path)
				}
			}
			cfg = newCfg
		}
		svc.UpdateConfig(cfg)

		if cleanupCancel := activeCancel.Take(); cleanupCancel != nil {
			cleanupCancel()
		}
		fmt.Println()
	}

	return nil
}

// handleSlashCommand processes slash commands. Returns true if the input was handled.
func handleSlashCommand(input string, cfg *config.Config, svc *service.Service, cmd *cobra.Command, authStatus *config.ClaudeAuthStatus) bool {
	parts := strings.SplitN(input, " ", 2)
	command := strings.ToLower(parts[0])
	arg := ""
	if len(parts) > 1 {
		arg = strings.TrimSpace(parts[1])
	}

	switch command {
	case "/quit", "/exit":
		terminal.Info("Goodbye!")
		os.Exit(0)
		return true

	case "/help":
		printHelp()
		return true

	case "/model":
		if arg == "" {
			picked := terminal.Pick("Models", []terminal.PickerOption{
				{Label: "sonnet", Desc: "Claude Sonnet 4.6 — fast, great for most tasks (default)"},
				{Label: "opus", Desc: "Claude Opus 4.6 — most capable, slower"},
				{Label: "haiku", Desc: "Claude Haiku 4.5 — fastest, lightweight tasks"},
			}, svc.CurrentModel())
			if picked != "" {
				svc.SetModel(picked)
				terminal.Success(fmt.Sprintf("Model set to %s", picked))
			}
			fmt.Println()
		} else {
			svc.SetModel(arg)
			terminal.Success(fmt.Sprintf("Model set to %s", arg))
			fmt.Println()
		}
		return true

	case "/clear":
		svc.ClearSession()
		terminal.Success("Session cleared")
		fmt.Println()
		return true

	case "/run":
		if !requireProjectForSlashCommand(cfg) {
			return true
		}
		if err := runWithSlashCommandContext(cmd, svc.Run); err != nil {
			terminal.Error(fmt.Sprintf("Run failed: %v", err))
		}
		fmt.Println()
		return true

	case "/simulator":
		handleSimulatorCommand(arg, svc)
		return true

	case "/fix":
		if !requireProjectForSlashCommand(cfg) {
			return true
		}
		if err := runWithSlashCommandContext(cmd, svc.Fix); err != nil {
			terminal.Error(fmt.Sprintf("Fix failed: %v", err))
		}
		fmt.Println()
		return true

	case "/ask":
		if !requireProjectForSlashCommand(cfg) {
			return true
		}
		if arg == "" {
			terminal.Warning("Usage: /ask <question>")
			fmt.Println()
			return true
		}
		usageBefore := svc.Usage()
		if err := runWithSlashCommandContext(cmd, func(ctx context.Context) error {
			return svc.Ask(ctx, arg)
		}); err != nil {
			terminal.Error(fmt.Sprintf("Ask failed: %v", err))
		} else {
			usageAfter := svc.Usage()
			if usageAfter.Requests > usageBefore.Requests {
				costDelta := usageAfter.TotalCostUSD - usageBefore.TotalCostUSD
				fmt.Printf("  %s$%.4f%s\n", terminal.Dim, costDelta, terminal.Reset)
			}
		}
		fmt.Println()
		return true

	case "/open":
		if !requireProjectForSlashCommand(cfg) {
			return true
		}
		if err := svc.Open(); err != nil {
			terminal.Error(fmt.Sprintf("Failed to open project: %v", err))
		}
		fmt.Println()
		return true

	case "/usage":
		printUsage(svc)
		return true

	case "/info":
		svc.Info()
		// Append auth + usage info
		if authStatus != nil && authStatus.LoggedIn && authStatus.Email != "" {
			planLabel := authStatus.SubscriptionType
			if planLabel != "" {
				planLabel = strings.ToUpper(planLabel[:1]) + planLabel[1:] + " plan"
				terminal.Detail("Claude", fmt.Sprintf("%s (%s)", authStatus.Email, planLabel))
			} else {
				terminal.Detail("Claude", authStatus.Email)
			}
		}
		usage := svc.Usage()
		if usage.Requests > 0 {
			terminal.Detail("Session cost", fmt.Sprintf("$%.2f (%d requests)", usage.TotalCostUSD, usage.Requests))
		}
		fmt.Println()
		return true

	case "/projects":
		return false // sentinel — handled in main loop

	case "/setup":
		if err := setupCmd.RunE(cmd, nil); err != nil {
			terminal.Error(fmt.Sprintf("Setup failed: %v", err))
		}
		fmt.Println()
		return true

	case "/integrations":
		RunIntegrationsInteractive()
		return true

	default:
		terminal.Warning(fmt.Sprintf("Unknown command: %s. Type /help for available commands.", command))
		fmt.Println()
		return true
	}
}

func requireProjectForSlashCommand(cfg *config.Config) bool {
	if cfg.HasProject() {
		return true
	}
	terminal.Error("No project found. Build an app first.")
	fmt.Println()
	return false
}

func runWithSlashCommandContext(cmd *cobra.Command, fn func(context.Context) error) error {
	ctx, cancel := context.WithCancel(cmd.Context())
	defer cancel()
	return fn(ctx)
}

// handleSimulatorCommand processes the /simulator command.
func handleSimulatorCommand(arg string, svc *service.Service) {
	devices, err := svc.ListSimulators()
	if err != nil {
		terminal.Error(fmt.Sprintf("Failed to list simulators: %v", err))
		fmt.Println()
		return
	}

	if len(devices) == 0 {
		terminal.Error("No iPhone simulators available. Install them via Xcode.")
		fmt.Println()
		return
	}

	current := svc.CurrentSimulator()

	if arg == "" {
		// Interactive picker
		var opts []terminal.PickerOption
		for _, d := range devices {
			opts = append(opts, terminal.PickerOption{Label: d.Name, Desc: d.Runtime})
		}
		picked := terminal.Pick("Simulators", opts, current)
		if picked != "" {
			svc.SetSimulator(picked)
			terminal.Success(fmt.Sprintf("Simulator set to %s", picked))
		}
		fmt.Println()
		return
	}

	// Try to match by number
	if n, err := strconv.Atoi(arg); err == nil {
		if n >= 1 && n <= len(devices) {
			selected := devices[n-1].Name
			svc.SetSimulator(selected)
			terminal.Success(fmt.Sprintf("Simulator set to %s", selected))
			fmt.Println()
			return
		}
	}

	// Try to match by name (case-insensitive prefix)
	argLower := strings.ToLower(arg)
	for _, d := range devices {
		if strings.ToLower(d.Name) == argLower || strings.HasPrefix(strings.ToLower(d.Name), argLower) {
			svc.SetSimulator(d.Name)
			terminal.Success(fmt.Sprintf("Simulator set to %s", d.Name))
			fmt.Println()
			return
		}
	}

	terminal.Error(fmt.Sprintf("Unknown simulator: %s", arg))
	terminal.Info("Use /simulator to see available devices.")
	fmt.Println()
}

func printUsage(svc *service.Service) {
	usage := svc.Usage()
	fmt.Println()
	terminal.Header("Session Usage")
	terminal.Divider()
	terminal.Detail("Requests", fmt.Sprintf("%d", usage.Requests))
	terminal.Detail("Input tokens", fmt.Sprintf("%s", storage.FormatTokenCount(usage.InputTokens)))
	terminal.Detail("Output tokens", fmt.Sprintf("%s", storage.FormatTokenCount(usage.OutputTokens)))
	if usage.CacheRead > 0 {
		terminal.Detail("Cache read", fmt.Sprintf("%s", storage.FormatTokenCount(usage.CacheRead)))
	}
	if usage.CacheCreated > 0 {
		terminal.Detail("Cache created", fmt.Sprintf("%s", storage.FormatTokenCount(usage.CacheCreated)))
	}
	terminal.Detail("Total cost", fmt.Sprintf("$%.2f", usage.TotalCostUSD))
	fmt.Println()
}

// showProjectPicker displays the project selection picker.
// Returns the selected ProjectInfo, or nil if "New project" was chosen.
func showProjectPicker(projects []config.ProjectInfo) *config.ProjectInfo {
	opts := []terminal.PickerOption{
		{Label: "New project", Desc: "Start a new app"},
	}
	for _, p := range projects {
		opts = append(opts, terminal.PickerOption{
			Label: p.Name,
			Desc:  "Created " + timeAgo(p.CreatedAt),
		})
	}

	fmt.Printf("  %sYour projects:%s\n", terminal.Dim, terminal.Reset)
	picked := terminal.Pick("", opts, "")
	if picked == "" || picked == "New project" {
		return nil
	}
	for i := range projects {
		if projects[i].Name == picked {
			return &projects[i]
		}
	}
	return nil
}

// timeAgo returns a human-readable relative time string.
func timeAgo(t time.Time) string {
	d := time.Since(t)
	switch {
	case d < time.Minute:
		return "just now"
	case d < time.Hour:
		m := int(d.Minutes())
		if m == 1 {
			return "1 minute ago"
		}
		return fmt.Sprintf("%d minutes ago", m)
	case d < 24*time.Hour:
		h := int(d.Hours())
		if h == 1 {
			return "1 hour ago"
		}
		return fmt.Sprintf("%d hours ago", h)
	case d < 7*24*time.Hour:
		days := int(d.Hours() / 24)
		if days == 1 {
			return "1 day ago"
		}
		return fmt.Sprintf("%d days ago", days)
	case d < 30*24*time.Hour:
		weeks := int(d.Hours() / 24 / 7)
		if weeks == 1 {
			return "1 week ago"
		}
		return fmt.Sprintf("%d weeks ago", weeks)
	default:
		months := int(d.Hours() / 24 / 30)
		if months == 1 {
			return "1 month ago"
		}
		return fmt.Sprintf("%d months ago", months)
	}
}

func printHelp() {
	fmt.Println()
	terminal.Header("Commands")
	fmt.Printf("  %s/run%s              Build and launch in simulator%s\n", terminal.Bold, terminal.Reset+terminal.Dim, terminal.Reset)
	fmt.Printf("  %s/simulator [name]%s Select simulator device%s\n", terminal.Bold, terminal.Reset+terminal.Dim, terminal.Reset)
	fmt.Printf("  %s/model [name]%s     Show or switch model (sonnet, opus, haiku)%s\n", terminal.Bold, terminal.Reset+terminal.Dim, terminal.Reset)
	fmt.Printf("  %s/fix%s              Auto-fix build errors%s\n", terminal.Bold, terminal.Reset+terminal.Dim, terminal.Reset)
	fmt.Printf("  %s/ask <question>%s  Ask about your project (cheap, read-only)%s\n", terminal.Bold, terminal.Reset+terminal.Dim, terminal.Reset)
	fmt.Printf("  %s/open%s             Open project in Xcode%s\n", terminal.Bold, terminal.Reset+terminal.Dim, terminal.Reset)
	fmt.Printf("  %s/projects%s         Switch to another project%s\n", terminal.Bold, terminal.Reset+terminal.Dim, terminal.Reset)
	fmt.Printf("  %s/info%s             Show project info%s\n", terminal.Bold, terminal.Reset+terminal.Dim, terminal.Reset)
	fmt.Printf("  %s/usage%s            Show token usage and costs%s\n", terminal.Bold, terminal.Reset+terminal.Dim, terminal.Reset)
	fmt.Printf("  %s/clear%s            Clear conversation session%s\n", terminal.Bold, terminal.Reset+terminal.Dim, terminal.Reset)
	fmt.Printf("  %s/setup%s            Install prerequisites%s\n", terminal.Bold, terminal.Reset+terminal.Dim, terminal.Reset)
	fmt.Printf("  %s/integrations%s    Manage backend integrations%s\n", terminal.Bold, terminal.Reset+terminal.Dim, terminal.Reset)
	fmt.Printf("  %s/help%s             Show this help%s\n", terminal.Bold, terminal.Reset+terminal.Dim, terminal.Reset)
	fmt.Printf("  %s/quit%s             Exit session%s\n", terminal.Bold, terminal.Reset+terminal.Dim, terminal.Reset)
	fmt.Println()
	fmt.Printf("  %sJust type a description and press Enter to submit.%s\n", terminal.Dim, terminal.Reset)
	fmt.Printf("  %sEsc+Enter for newline. Drag image files to attach.%s\n", terminal.Dim, terminal.Reset)
	fmt.Println()
}
