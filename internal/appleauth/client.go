package appleauth

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/http/cookiejar"
)

const (
	idmsaBase   = "https://idmsa.apple.com/appleauth/auth"
	olympusURL  = "https://appstoreconnect.apple.com/olympus/v1/app/config?hostname=itunesconnect.apple.com"
	userAgentUA = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.0 Safari/605.1.15"
)

// Client handles Apple IDMSA SRP authentication and 2FA.
type Client struct {
	httpClient  *http.Client
	serviceKey  string
	sessionID   string
	scnt        string
	csrfTokens  map[string]string
	sessionData []byte
}

// NewClient creates a new Apple authentication client.
func NewClient() (*Client, error) {
	jar, err := cookiejar.New(nil)
	if err != nil {
		return nil, err
	}
	return &Client{
		httpClient: &http.Client{Jar: jar},
	}, nil
}

// CookieJar returns the HTTP cookie jar for session persistence.
func (c *Client) CookieJar() http.CookieJar {
	return c.httpClient.Jar
}

// FetchServiceKey fetches the Apple auth service key from Olympus.
func (c *Client) FetchServiceKey(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, "GET", olympusURL, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", userAgentUA)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("olympus request failed: %w", err)
	}
	defer resp.Body.Close()

	var config struct {
		AuthServiceKey string `json:"authServiceKey"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&config); err != nil {
		return fmt.Errorf("failed to parse olympus config: %w", err)
	}
	if config.AuthServiceKey == "" {
		return fmt.Errorf("no authServiceKey in olympus config")
	}
	c.serviceKey = config.AuthServiceKey
	log.Printf("[appleauth] serviceKey=%s", c.serviceKey)
	return nil
}

type srpInitResponse struct {
	Salt      string `json:"salt"`
	B         string `json:"b"`
	C         string `json:"c"`
	Iteration int    `json:"iteration"`
	Protocol  string `json:"protocol"`
}

// SignIn performs SRP authentication with Apple ID.
// Returns an AuthState if 2FA is required, nil if sign-in completed directly.
func (c *Client) SignIn(ctx context.Context, appleID, password string) (*AuthState, error) {
	log.Printf("[appleauth] SignIn: starting for %s", appleID)

	a, A := srpGenerateClientKeyPair()

	initBody := map[string]any{
		"a":           encodeBase64(A.Bytes()),
		"accountName": appleID,
		"protocols":   []string{"s2k", "s2k_fo"},
	}
	initJSON, _ := json.Marshal(initBody)

	initReq, err := http.NewRequestWithContext(ctx, "POST", idmsaBase+"/signin/init", bytes.NewReader(initJSON))
	if err != nil {
		return nil, err
	}
	c.setHeaders(initReq)

	log.Printf("[appleauth] SignIn: POST /signin/init")
	initResp, err := c.httpClient.Do(initReq)
	if err != nil {
		return nil, fmt.Errorf("signin/init failed: %w", err)
	}
	defer initResp.Body.Close()

	if initResp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(initResp.Body)
		log.Printf("[appleauth] SignIn: init error %d: %s", initResp.StatusCode, string(body))
		return nil, fmt.Errorf("signin/init returned %d: %s", initResp.StatusCode, string(body))
	}

	var initResult srpInitResponse
	if err := json.NewDecoder(initResp.Body).Decode(&initResult); err != nil {
		return nil, fmt.Errorf("failed to parse init response: %w", err)
	}
	log.Printf("[appleauth] SignIn: protocol=%s iterations=%d", initResult.Protocol, initResult.Iteration)

	salt, err := decodeBase64(initResult.Salt)
	if err != nil {
		return nil, fmt.Errorf("invalid salt: %w", err)
	}
	B := newBigIntFromBase64(initResult.B)
	if B == nil {
		return nil, fmt.Errorf("invalid B value")
	}

	m1, m2, err := computeSRP(a, A, B, salt, appleID, password, initResult.Protocol, initResult.Iteration)
	if err != nil {
		return nil, fmt.Errorf("SRP computation failed: %w", err)
	}

	completeBody := map[string]any{
		"accountName": appleID,
		"m1":          encodeBase64(m1),
		"m2":          encodeBase64(m2),
		"c":           initResult.C,
		"rememberMe":  true,
	}
	completeJSON, _ := json.Marshal(completeBody)

	completeReq, err := http.NewRequestWithContext(ctx, "POST", idmsaBase+"/signin/complete?isRememberMeEnabled=true", bytes.NewReader(completeJSON))
	if err != nil {
		return nil, err
	}
	c.setHeaders(completeReq)

	log.Printf("[appleauth] SignIn: POST /signin/complete")
	completeResp, err := c.httpClient.Do(completeReq)
	if err != nil {
		return nil, fmt.Errorf("signin/complete failed: %w", err)
	}
	defer completeResp.Body.Close()

	if v := completeResp.Header.Get("X-Apple-ID-Session-Id"); v != "" {
		c.sessionID = v
	}
	if v := completeResp.Header.Get("scnt"); v != "" {
		c.scnt = v
	}

	body, _ := io.ReadAll(completeResp.Body)
	log.Printf("[appleauth] SignIn: complete status=%d sessionID=%s scnt=%s",
		completeResp.StatusCode, truncateForLog(c.sessionID, 8), truncateForLog(c.scnt, 8))

	switch completeResp.StatusCode {
	case http.StatusOK:
		log.Printf("[appleauth] SignIn: success — no 2FA required")
		return nil, nil

	case http.StatusConflict: // 409 = 2FA required
		log.Printf("[appleauth] SignIn: 2FA required")
		authState, fetchErr := c.fetchAuthOptions(ctx)
		if fetchErr != nil {
			log.Printf("[appleauth] SignIn: failed to fetch auth options: %v", fetchErr)
			return &AuthState{CodeLength: 6, HasTrustedDevices: true}, nil
		}
		return authState, nil

	case http.StatusForbidden:
		return nil, fmt.Errorf("account locked — visit iforgot.apple.com to unlock")

	case http.StatusUnauthorized:
		return nil, fmt.Errorf("incorrect Apple ID or password")

	default:
		return nil, fmt.Errorf("signin/complete returned %d: %s", completeResp.StatusCode, string(body))
	}
}

// RequestSMSCode sends an SMS verification code to the specified phone.
func (c *Client) RequestSMSCode(ctx context.Context, phoneID int) error {
	body := map[string]any{
		"phoneNumber": map[string]any{"id": phoneID},
		"mode":        "sms",
	}
	bodyJSON, _ := json.Marshal(body)

	req, err := http.NewRequestWithContext(ctx, "PUT", idmsaBase+"/verify/phone", bytes.NewReader(bodyJSON))
	if err != nil {
		return err
	}
	c.setHeaders(req)

	log.Printf("[appleauth] RequestSMSCode: phoneID=%d", phoneID)
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("SMS request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("SMS request returned %d: %s", resp.StatusCode, string(respBody))
	}
	log.Printf("[appleauth] RequestSMSCode: success")
	return nil
}

// VerifyDeviceCode verifies a trusted device OTP code.
func (c *Client) VerifyDeviceCode(ctx context.Context, code string) error {
	body := map[string]any{
		"securityCode": map[string]string{"code": code},
	}
	bodyJSON, _ := json.Marshal(body)

	req, err := http.NewRequestWithContext(ctx, "POST", idmsaBase+"/verify/trusteddevice/securitycode", bytes.NewReader(bodyJSON))
	if err != nil {
		return err
	}
	c.setHeaders(req)

	log.Printf("[appleauth] VerifyDeviceCode: verifying")
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("device verification failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNoContent && resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		if resp.StatusCode == http.StatusForbidden {
			return fmt.Errorf("invalid verification code, please try again")
		}
		return fmt.Errorf("device verification returned %d: %s", resp.StatusCode, string(respBody))
	}

	if v := resp.Header.Get("X-Apple-ID-Session-Id"); v != "" {
		c.sessionID = v
	}
	if v := resp.Header.Get("scnt"); v != "" {
		c.scnt = v
	}
	log.Printf("[appleauth] VerifyDeviceCode: success")
	return nil
}

// VerifySMSCode verifies an SMS OTP code.
func (c *Client) VerifySMSCode(ctx context.Context, code string, phoneID int) error {
	body := map[string]any{
		"securityCode": map[string]string{"code": code},
		"phoneNumber":  map[string]any{"id": phoneID},
		"mode":         "sms",
	}
	bodyJSON, _ := json.Marshal(body)

	req, err := http.NewRequestWithContext(ctx, "POST", idmsaBase+"/verify/phone/securitycode", bytes.NewReader(bodyJSON))
	if err != nil {
		return err
	}
	c.setHeaders(req)

	log.Printf("[appleauth] VerifySMSCode: phoneID=%d", phoneID)
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("SMS verification failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNoContent && resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		if resp.StatusCode == http.StatusForbidden {
			return fmt.Errorf("invalid verification code, please try again")
		}
		return fmt.Errorf("SMS verification returned %d: %s", resp.StatusCode, string(respBody))
	}

	if v := resp.Header.Get("X-Apple-ID-Session-Id"); v != "" {
		c.sessionID = v
	}
	if v := resp.Header.Get("scnt"); v != "" {
		c.scnt = v
	}
	log.Printf("[appleauth] VerifySMSCode: success")
	return nil
}

// TrustSession establishes a trusted session after 2FA verification.
func (c *Client) TrustSession(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, "GET", idmsaBase+"/2sv/trust", nil)
	if err != nil {
		return err
	}
	c.setHeaders(req)

	log.Printf("[appleauth] TrustSession: GET /2sv/trust")
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("trust session failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNoContent && resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("trust session returned %d: %s", resp.StatusCode, string(respBody))
	}
	log.Printf("[appleauth] TrustSession: success")

	if err := c.fetchOlympusSession(ctx); err != nil {
		log.Printf("[appleauth] TrustSession: olympus session fetch failed (non-fatal): %v", err)
	}
	return nil
}

func (c *Client) fetchOlympusSession(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, "GET", "https://appstoreconnect.apple.com/olympus/v1/session", nil)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", userAgentUA)

	log.Printf("[appleauth] fetchOlympusSession: GET /olympus/v1/session")
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("olympus session request failed: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	log.Printf("[appleauth] fetchOlympusSession: status=%d bodyLen=%d", resp.StatusCode, len(body))

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("olympus session returned %d", resp.StatusCode)
	}

	c.csrfTokens = make(map[string]string)
	for _, key := range []string{"csrf", "csrf_ts"} {
		if v := resp.Header.Get(key); v != "" {
			c.csrfTokens[key] = v
		}
	}
	c.sessionData = body
	return nil
}

func (c *Client) setHeaders(req *http.Request) {
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", userAgentUA)
	if c.serviceKey != "" {
		req.Header.Set("X-Apple-Widget-Key", c.serviceKey)
	}
	if c.sessionID != "" {
		req.Header.Set("X-Apple-ID-Session-Id", c.sessionID)
	}
	if c.scnt != "" {
		req.Header.Set("scnt", c.scnt)
	}
}

func (c *Client) fetchAuthOptions(ctx context.Context) (*AuthState, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", idmsaBase, nil)
	if err != nil {
		return nil, err
	}
	c.setHeaders(req)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("auth options request failed: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	var authResp struct {
		TrustedPhoneNumbers []TrustedPhone `json:"trustedPhoneNumbers"`
		TrustedDevices      []struct {
			ID   string `json:"id"`
			Name string `json:"name"`
		} `json:"trustedDevices"`
		SecurityCode struct {
			Length int `json:"length"`
		} `json:"securityCode"`
	}
	if err := json.Unmarshal(body, &authResp); err != nil {
		return nil, fmt.Errorf("failed to parse auth options: %w", err)
	}

	codeLength := 6
	if authResp.SecurityCode.Length > 0 {
		codeLength = authResp.SecurityCode.Length
	}

	state := &AuthState{
		TrustedPhones:     authResp.TrustedPhoneNumbers,
		CodeLength:        codeLength,
		HasTrustedDevices: len(authResp.TrustedDevices) > 0,
	}
	log.Printf("[appleauth] fetchAuthOptions: phones=%d codeLen=%d hasTrustedDevices=%v",
		len(state.TrustedPhones), state.CodeLength, state.HasTrustedDevices)
	return state, nil
}

func truncateForLog(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
