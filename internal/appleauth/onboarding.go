package appleauth

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"syscall"
	"time"
)

// RunOnboarding executes the full onboarding flow after web authentication:
//  1. Use session cookies from IDMSA auth
//  2. GET /iris/v1/contentProviders -> issuer ID + team name
//  3. POST /iris/v1/apiKeys -> key ID + .p8 content
//  4. asc auth login -> register the key for CLI usage
func RunOnboarding(ctx context.Context, client *Client, appleID string) (*OnboardingResult, error) {
	log.Printf("[appleauth] RunOnboarding: starting for %s", appleID)

	jar := client.CookieJar()
	httpClient := &http.Client{Jar: jar, Timeout: 30 * time.Second}

	const irisBase = "https://appstoreconnect.apple.com/iris/v1"

	csrfTokens := client.csrfTokens
	setIrisHeaders := func(req *http.Request) {
		req.Header.Set("Accept", "application/vnd.api+json, application/json")
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("x-csrf-itc", "[asc-ui]")
		for k, v := range csrfTokens {
			req.Header.Set(k, v)
		}
	}

	// Step 1: Get content providers (issuer ID + team name)
	log.Printf("[appleauth] RunOnboarding: fetching content providers")
	providerReq, err := http.NewRequestWithContext(ctx, "GET", irisBase+"/contentProviders", nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	setIrisHeaders(providerReq)

	providerResp, err := httpClient.Do(providerReq)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch content providers: %w", err)
	}
	defer providerResp.Body.Close()

	log.Printf("[appleauth] RunOnboarding: content providers status=%d", providerResp.StatusCode)

	var issuerID, teamName string

	if providerResp.StatusCode == http.StatusOK {
		type irisContentProvider struct {
			Data []struct {
				ID         string `json:"id"`
				Attributes struct {
					Name string `json:"name"`
				} `json:"attributes"`
			} `json:"data"`
		}
		var providers irisContentProvider
		if err := json.NewDecoder(providerResp.Body).Decode(&providers); err != nil {
			return nil, fmt.Errorf("failed to parse content providers: %w", err)
		}
		if len(providers.Data) == 0 {
			return nil, fmt.Errorf("no content providers found for this account")
		}
		issuerID = providers.Data[0].ID
		teamName = providers.Data[0].Attributes.Name
		log.Printf("[appleauth] RunOnboarding: issuerID=%s teamName=%s", issuerID, teamName)
	} else {
		respBody, _ := io.ReadAll(providerResp.Body)
		log.Printf("[appleauth] RunOnboarding: contentProviders failed %d: %s", providerResp.StatusCode, string(respBody))

		if client.sessionData == nil || len(client.sessionData) == 0 {
			return nil, fmt.Errorf("content providers request failed (%d) and no session data available", providerResp.StatusCode)
		}

		var session struct {
			Provider struct {
				Name     string `json:"name"`
				PublicID string `json:"publicProviderId"`
			} `json:"provider"`
		}
		if err := json.Unmarshal(client.sessionData, &session); err != nil {
			return nil, fmt.Errorf("failed to parse olympus session: %w", err)
		}
		if session.Provider.PublicID == "" {
			return nil, fmt.Errorf("olympus session has no publicProviderId")
		}
		issuerID = session.Provider.PublicID
		teamName = session.Provider.Name
		log.Printf("[appleauth] RunOnboarding: issuerID=%s teamName=%s (from olympus fallback)", issuerID, teamName)
	}

	// Step 2: Create API key
	nickname := fmt.Sprintf("nanowave-%d", time.Now().Unix())
	log.Printf("[appleauth] RunOnboarding: creating API key nickname=%s", nickname)
	keyBody := map[string]any{
		"data": map[string]any{
			"type": "apiKeys",
			"attributes": map[string]any{
				"nickname":       nickname,
				"roles":          []string{"ADMIN"},
				"allAppsVisible": true,
				"keyType":        "PUBLIC_API",
			},
		},
	}
	keyJSON, _ := json.Marshal(keyBody)

	keyReq, err := http.NewRequestWithContext(ctx, "POST", irisBase+"/apiKeys", bytes.NewReader(keyJSON))
	if err != nil {
		return nil, fmt.Errorf("failed to create API key request: %w", err)
	}
	keyReq.Header.Set("Content-Type", "application/json")
	setIrisHeaders(keyReq)

	keyResp, err := httpClient.Do(keyReq)
	if err != nil {
		return nil, fmt.Errorf("failed to create API key: %w", err)
	}
	defer keyResp.Body.Close()

	log.Printf("[appleauth] RunOnboarding: API key response status=%d", keyResp.StatusCode)

	if keyResp.StatusCode == http.StatusForbidden {
		return nil, fmt.Errorf("your account needs Admin role to create API keys")
	}
	if keyResp.StatusCode != http.StatusOK && keyResp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(keyResp.Body)
		return nil, fmt.Errorf("API key creation failed (%d): %s", keyResp.StatusCode, string(body))
	}

	type irisAPIKeyResponse struct {
		Data struct {
			ID         string `json:"id"`
			Attributes struct {
				Nickname   string `json:"nickname"`
				PrivateKey string `json:"privateKey"`
			} `json:"attributes"`
		} `json:"data"`
	}

	var keyResult irisAPIKeyResponse
	if err := json.NewDecoder(keyResp.Body).Decode(&keyResult); err != nil {
		return nil, fmt.Errorf("failed to parse API key response: %w", err)
	}

	keyID := keyResult.Data.ID
	log.Printf("[appleauth] RunOnboarding: keyID=%s", keyID)

	if keyID == "" {
		return nil, fmt.Errorf("API key created but no key ID returned")
	}

	// Download the private key
	privateKey := keyResult.Data.Attributes.PrivateKey
	if privateKey == "" {
		log.Printf("[appleauth] RunOnboarding: downloading private key for %s", keyID)
		dlURL := irisBase + "/apiKeys/" + keyID + "?fields%5BapiKeys%5D=privateKey"
		dlReq, err := http.NewRequestWithContext(ctx, "GET", dlURL, nil)
		if err != nil {
			return nil, fmt.Errorf("failed to create private key download request: %w", err)
		}
		setIrisHeaders(dlReq)

		dlResp, err := httpClient.Do(dlReq)
		if err != nil {
			return nil, fmt.Errorf("failed to download private key: %w", err)
		}
		defer dlResp.Body.Close()

		dlBody, _ := io.ReadAll(dlResp.Body)
		if dlResp.StatusCode != http.StatusOK {
			return nil, fmt.Errorf("private key download failed (%d): %s", dlResp.StatusCode, string(dlBody))
		}

		var dlResult irisAPIKeyResponse
		if err := json.Unmarshal(dlBody, &dlResult); err != nil {
			return nil, fmt.Errorf("failed to parse private key response: %w", err)
		}

		encoded := dlResult.Data.Attributes.PrivateKey
		if encoded == "" {
			return nil, fmt.Errorf("private key field is empty — one-time download may have already been used")
		}

		decoded, err := base64.StdEncoding.DecodeString(encoded)
		if err != nil {
			if strings.Contains(encoded, "BEGIN PRIVATE KEY") {
				privateKey = encoded
			} else {
				return nil, fmt.Errorf("failed to decode private key: %w", err)
			}
		} else {
			privateKey = string(decoded)
		}
		log.Printf("[appleauth] RunOnboarding: privateKeyLen=%d", len(privateKey))
	}

	if privateKey == "" {
		return nil, fmt.Errorf("key created but private key download failed")
	}

	// Step 3: Register the key with the asc CLI
	log.Printf("[appleauth] RunOnboarding: registering key with asc CLI")
	tmpFile, err := os.CreateTemp("", "nanowave-onboard-*.p8")
	if err != nil {
		return nil, fmt.Errorf("failed to create temp file for key: %w", err)
	}
	tmpPath := tmpFile.Name()
	defer os.Remove(tmpPath)

	if _, err := tmpFile.WriteString(privateKey); err != nil {
		tmpFile.Close()
		return nil, fmt.Errorf("failed to write key file: %w", err)
	}
	tmpFile.Close()

	if err := os.Chmod(tmpPath, 0o600); err != nil {
		return nil, fmt.Errorf("failed to set key file permissions: %w", err)
	}

	cmd := exec.CommandContext(ctx, "asc", "auth", "login",
		"--name", nickname,
		"--key-id", keyID,
		"--issuer-id", issuerID,
		"--private-key", tmpPath,
	)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	output, cmdErr := cmd.CombinedOutput()
	log.Printf("[appleauth] RunOnboarding: asc auth login: %s (err=%v)", strings.TrimSpace(string(output)), cmdErr)
	if cmdErr != nil {
		return nil, fmt.Errorf("key registration failed: %s", strings.TrimSpace(string(output)))
	}

	// Save iris cookies for future use
	if err := SaveIrisCookies(appleID, jar); err != nil {
		log.Printf("[appleauth] RunOnboarding: failed to save iris cookies: %v", err)
	}

	log.Printf("[appleauth] RunOnboarding: complete — keyID=%s issuerID=%s team=%s", keyID, issuerID, teamName)
	return &OnboardingResult{KeyID: keyID, IssuerID: issuerID, TeamName: teamName}, nil
}
