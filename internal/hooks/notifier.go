package hooks

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"strings"
)

// NotifyTelegram sends a message via the Telegram Bot API.
func NotifyTelegram(ctx context.Context, botToken, chatID, message string, silent bool) error {
	url := fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage", botToken)

	payload := map[string]any{
		"chat_id":    chatID,
		"text":       message,
		"parse_mode": "Markdown",
	}
	if silent {
		payload["disable_notification"] = true
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("telegram: marshal payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("telegram: create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("telegram: send: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return fmt.Errorf("telegram: unexpected status %d: %s", resp.StatusCode, strings.TrimSpace(string(respBody)))
	}

	return nil
}

// NotifySlack sends a message via a Slack incoming webhook.
func NotifySlack(ctx context.Context, webhookURL, message string) error {
	payload := map[string]string{
		"text": message,
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("slack: marshal payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", webhookURL, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("slack: create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("slack: send: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return fmt.Errorf("slack: unexpected status %d: %s", resp.StatusCode, strings.TrimSpace(string(respBody)))
	}

	return nil
}

// ResolveBotToken resolves a Telegram bot token from: env var, macOS keychain, or error.
func ResolveBotToken(envVar, keychainService string) (string, error) {
	// 1. Environment variable
	if envVar != "" {
		if val := os.Getenv(envVar); val != "" {
			return val, nil
		}
	}

	// 2. macOS Keychain
	if keychainService != "" {
		token, err := keychainLookup(keychainService)
		if err == nil && token != "" {
			return token, nil
		}
	}

	return "", fmt.Errorf("bot token not found: set %s env var or add to keychain as %s", envVar, keychainService)
}

// ResolveWebhookURL resolves a Slack webhook URL from an environment variable.
func ResolveWebhookURL(envVar string) (string, error) {
	if envVar != "" {
		if val := os.Getenv(envVar); val != "" {
			return val, nil
		}
	}
	return "", fmt.Errorf("webhook URL not found: set %s env var", envVar)
}

func keychainLookup(service string) (string, error) {
	cmd := exec.Command("security", "find-generic-password", "-s", service, "-w")
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("keychain lookup %s: %w", service, err)
	}
	return strings.TrimSpace(string(out)), nil
}
