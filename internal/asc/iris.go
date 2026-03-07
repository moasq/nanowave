package asc

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"
)

// IrisErrorKind classifies Iris API failures.
type IrisErrorKind int

const (
	IrisErrorUnknown       IrisErrorKind = iota
	IrisErrorSessionExpired              // 401/403 — Apple ID session is stale
	IrisErrorNameCollision               // 409 — app name already taken
)

// IrisError is a typed error returned by Iris API calls.
type IrisError struct {
	Kind       IrisErrorKind
	StatusCode int
	Detail     string // human-readable detail from Apple's response
}

func (e *IrisError) Error() string { return e.Detail }

// IsIrisSessionExpired checks whether an error is an Iris session expiry.
func IsIrisSessionExpired(err error) bool {
	var ie *IrisError
	return errors.As(err, &ie) && ie.Kind == IrisErrorSessionExpired
}

// IsIrisNameCollision checks whether an error is an app name collision.
func IsIrisNameCollision(err error) bool {
	var ie *IrisError
	return errors.As(err, &ie) && ie.Kind == IrisErrorNameCollision
}

// VerifyIrisSession checks whether the Iris session cookies are still valid
// by making a lightweight request to the App Store Connect API.
func VerifyIrisSession(ctx context.Context, jar http.CookieJar) bool {
	httpClient := &http.Client{Jar: jar, Timeout: 15 * time.Second}

	req, err := http.NewRequestWithContext(ctx, "GET", "https://appstoreconnect.apple.com/iris/v1/apps?limit=1", nil)
	if err != nil {
		return false
	}
	req.Header.Set("Accept", "application/vnd.api+json, application/json")
	req.Header.Set("x-csrf-itc", "[asc-ui]")

	resp, err := httpClient.Do(req)
	if err != nil {
		log.Printf("[asc] VerifyIrisSession: request failed: %v", err)
		return false
	}
	defer resp.Body.Close()

	valid := resp.StatusCode == http.StatusOK
	log.Printf("[asc] VerifyIrisSession: status=%d valid=%v", resp.StatusCode, valid)
	return valid
}

// CreateAppViaIris creates an app in App Store Connect using the iris web API
// with saved Apple ID session cookies. This bypasses the API key limitation
// that prevents app creation via the REST API.
func CreateAppViaIris(ctx context.Context, jar http.CookieJar, appName, bundleID, bundleIDResourceID string) (string, error) {
	httpClient := &http.Client{Jar: jar, Timeout: 30 * time.Second}

	log.Printf("[asc] CreateAppViaIris: appName=%q bundleID=%s resourceID=%s", appName, bundleID, bundleIDResourceID)

	// Build the payload matching Apple's internal format (same as fastlane spaceship).
	// The app name goes into appInfoLocalizations in the included array, NOT in data.attributes.
	// data.attributes only has sku, primaryLocale, bundleId.
	platform := "IOS"
	reqBody := map[string]any{
		"data": map[string]any{
			"type": "apps",
			"attributes": map[string]any{
				"sku":           bundleID,
				"primaryLocale": "en-US",
				"bundleId":      bundleID,
			},
			"relationships": map[string]any{
				"appStoreVersions": map[string]any{
					"data": []map[string]any{
						{"type": "appStoreVersions", "id": "${store-version-" + platform + "}"},
					},
				},
				"appInfos": map[string]any{
					"data": []map[string]any{
						{"type": "appInfos", "id": "${new-appInfo-id}"},
					},
				},
			},
		},
		"included": []map[string]any{
			{
				"type": "appInfos",
				"id":   "${new-appInfo-id}",
				"relationships": map[string]any{
					"appInfoLocalizations": map[string]any{
						"data": []map[string]any{
							{"type": "appInfoLocalizations", "id": "${new-appInfoLocalization-id}"},
						},
					},
				},
			},
			{
				"type": "appInfoLocalizations",
				"id":   "${new-appInfoLocalization-id}",
				"attributes": map[string]any{
					"locale": "en-US",
					"name":   appName,
				},
			},
			{
				"type": "appStoreVersions",
				"id":   "${store-version-" + platform + "}",
				"attributes": map[string]any{
					"platform":      platform,
					"versionString": "1.0",
				},
				"relationships": map[string]any{
					"appStoreVersionLocalizations": map[string]any{
						"data": []map[string]any{
							{"type": "appStoreVersionLocalizations", "id": "${new-" + platform + "VersionLocalization-id}"},
						},
					},
				},
			},
			{
				"type": "appStoreVersionLocalizations",
				"id":   "${new-" + platform + "VersionLocalization-id}",
				"attributes": map[string]any{
					"locale": "en-US",
				},
			},
		},
	}
	bodyJSON, _ := json.Marshal(reqBody)

	createReq, err := http.NewRequestWithContext(ctx, "POST", "https://appstoreconnect.apple.com/iris/v1/apps", bytes.NewReader(bodyJSON))
	if err != nil {
		return "", err
	}
	createReq.Header.Set("Accept", "application/vnd.api+json, application/json")
	createReq.Header.Set("Content-Type", "application/json")
	createReq.Header.Set("x-csrf-itc", "[asc-ui]")

	createResp, err := httpClient.Do(createReq)
	if err != nil {
		return "", fmt.Errorf("app creation request failed: %w", err)
	}
	defer createResp.Body.Close()

	respBody, _ := io.ReadAll(createResp.Body)
	log.Printf("[asc] CreateAppViaIris: status=%d", createResp.StatusCode)

	if createResp.StatusCode != http.StatusCreated && createResp.StatusCode != http.StatusOK {
		detail := fmt.Sprintf("app creation returned HTTP %d", createResp.StatusCode)
		var errResp struct {
			Errors []struct {
				Detail string `json:"detail"`
			} `json:"errors"`
		}
		if json.Unmarshal(respBody, &errResp) == nil && len(errResp.Errors) > 0 {
			detail = errResp.Errors[0].Detail
		}

		kind := IrisErrorUnknown
		switch {
		case createResp.StatusCode == http.StatusUnauthorized || createResp.StatusCode == http.StatusForbidden:
			kind = IrisErrorSessionExpired
		case createResp.StatusCode == http.StatusConflict:
			kind = IrisErrorNameCollision
		}

		return "", &IrisError{Kind: kind, StatusCode: createResp.StatusCode, Detail: detail}
	}

	var result struct {
		Data struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return "", fmt.Errorf("failed to parse response: %w", err)
	}
	if result.Data.ID == "" {
		return "", fmt.Errorf("app created but no ID returned")
	}
	return result.Data.ID, nil
}
