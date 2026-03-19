package hooks

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// DryRunContext returns a context with dry-run mode enabled.
func DryRunContext(ctx context.Context) context.Context {
	return context.WithValue(ctx, dryRunKey, true)
}

func isDryRun(ctx context.Context) bool {
	v, _ := ctx.Value(dryRunKey).(bool)
	return v
}

// Fire loads the hook config, finds hooks matching the given event, and executes them.
// If no config exists, it returns nil silently.
func Fire(ctx context.Context, eventName string, vars map[string]string) error {
	cfg, err := LoadConfig()
	if err != nil {
		return fmt.Errorf("hooks: load config: %w", err)
	}
	if cfg == nil {
		return nil // no config, nothing to do
	}

	hooks, ok := cfg.Hooks[eventName]
	if !ok || len(hooks) == 0 {
		return nil
	}

	logDir := cfg.ResolveLogDir()
	_ = os.MkdirAll(logDir, 0o755)

	var firstErr error
	for _, hook := range hooks {
		if !shouldRun(hook, vars) {
			continue
		}

		hookErr := executeHook(ctx, cfg, hook, eventName, vars, logDir)
		if hookErr != nil {
			logHookExecution(logDir, eventName, hook.Name, "error", hookErr.Error())
			if !cfg.Defaults.ContinueOnError {
				return hookErr
			}
			if firstErr == nil {
				firstErr = hookErr
			}
		} else {
			logHookExecution(logDir, eventName, hook.Name, "success", "")
		}
	}

	return firstErr
}

func shouldRun(hook HookDefinition, vars map[string]string) bool {
	switch hook.When {
	case "always", "":
		return true
	case "success":
		return vars["STATUS"] == "success"
	case "failure":
		return vars["STATUS"] != "success"
	default:
		return true
	}
}

func executeHook(ctx context.Context, cfg *Config, hook HookDefinition, event string, vars map[string]string, logDir string) error {
	timeout := hook.TimeoutDuration(cfg.Defaults)
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	dryRun := isDryRun(ctx)

	if hook.Run != "" {
		return executeRunHook(ctx, hook, vars, dryRun)
	}
	if hook.Notify != "" {
		return executeNotifyHook(ctx, cfg, hook, vars, dryRun)
	}

	return fmt.Errorf("hook %q: no run or notify action defined", hook.Name)
}

func executeRunHook(ctx context.Context, hook HookDefinition, vars map[string]string, dryRun bool) error {
	command := substituteVars(hook.Run, vars)

	if dryRun {
		fmt.Fprintf(os.Stderr, "[dry-run] hook %q: would run: %s\n", hook.Name, command)
		return nil
	}

	fmt.Fprintf(os.Stderr, "[hooks] running %q: %s\n", hook.Name, command)

	shell, flags := resolveShell()
	args := append(flags, command)
	cmd := exec.CommandContext(ctx, shell, args...)
	cmd.Stdout = os.Stderr // user-facing output goes to stderr
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("hook %q: %w", hook.Name, err)
	}
	return nil
}

func executeNotifyHook(ctx context.Context, cfg *Config, hook HookDefinition, vars map[string]string, dryRun bool) error {
	message := substituteVars(hook.Template, vars)

	if dryRun {
		fmt.Fprintf(os.Stderr, "[dry-run] hook %q: would notify %s: %s\n", hook.Name, hook.Notify, message)
		return nil
	}

	fmt.Fprintf(os.Stderr, "[hooks] notifying %q via %s\n", hook.Name, hook.Notify)

	switch hook.Notify {
	case "telegram":
		nc, ok := cfg.Notifiers["telegram"]
		if !ok || !nc.Enabled {
			return fmt.Errorf("hook %q: telegram notifier not configured or disabled", hook.Name)
		}
		botToken, err := ResolveBotToken(nc.BotTokenEnv, nc.BotTokenKeychain)
		if err != nil {
			return fmt.Errorf("hook %q: %w", hook.Name, err)
		}
		chatID := nc.ChatID
		if chatID == "" {
			return fmt.Errorf("hook %q: telegram chat_id not configured", hook.Name)
		}
		return NotifyTelegram(ctx, botToken, chatID, message, false)

	case "slack":
		nc, ok := cfg.Notifiers["slack"]
		if !ok || !nc.Enabled {
			return fmt.Errorf("hook %q: slack notifier not configured or disabled", hook.Name)
		}
		webhookURL, err := ResolveWebhookURL(nc.WebhookURLEnv)
		if err != nil {
			return fmt.Errorf("hook %q: %w", hook.Name, err)
		}
		return NotifySlack(ctx, webhookURL, message)

	default:
		return fmt.Errorf("hook %q: unknown notifier %q", hook.Name, hook.Notify)
	}
}

// substituteVars replaces {{.KEY}} placeholders with values from vars.
func substituteVars(template string, vars map[string]string) string {
	result := template
	for key, value := range vars {
		result = strings.ReplaceAll(result, "{{."+key+"}}", value)
	}
	// Handle conditional templates: {{if eq .KEY "value"}}text{{else}}other{{end}}
	// Simple approach: just leave unresolved template syntax as-is
	return result
}

func resolveShell() (string, []string) {
	if _, err := exec.LookPath("bash"); err == nil {
		return "bash", []string{"-o", "pipefail", "-c"}
	}
	return "sh", []string{"-c"}
}

// FireSafe fires a hook event, logging any errors to stderr without returning them.
// Use this from embedded hook points where failures must never block the main operation.
func FireSafe(ctx context.Context, eventName string, vars map[string]string) {
	if err := Fire(ctx, eventName, vars); err != nil {
		fmt.Fprintf(os.Stderr, "[hooks] warning: %s: %v\n", eventName, err)
	}
}

func logHookExecution(logDir, event, hookName, status, errMsg string) {
	logFile := filepath.Join(logDir, "hooks.log")
	f, err := os.OpenFile(logFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return
	}
	defer f.Close()

	ts := time.Now().Format(time.RFC3339)
	line := fmt.Sprintf("%s event=%s hook=%s status=%s", ts, event, hookName, status)
	if errMsg != "" {
		line += " error=" + errMsg
	}
	fmt.Fprintln(f, line)
}
