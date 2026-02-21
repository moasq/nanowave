# App Store Connect API Key Setup

The App Store Connect API authenticates using JWT tokens signed with an API key. This document covers creating the key and generating tokens.

## Creating an API Key

1. Go to **App Store Connect → Users and Access → Integrations → App Store Connect API**
2. Click **Generate API Key**
3. Enter a name (e.g., "Nanowave CLI")
4. Select **Admin** role (required for certificate and profile management)
5. Click **Generate**

You'll get three values:

| Value | Example | Where to Find |
|---|---|---|
| **Key ID** | `ABC123DEFG` | Shown in the keys list |
| **Issuer ID** | `57246542-96fe-1a63-e053-0824d011072a` | Top of the API keys page |
| **Private Key (.p8)** | `AuthKey_ABC123DEFG.p8` | Download button (available only once!) |

> **Critical**: Download the `.p8` file immediately. Apple only lets you download it once. Store it securely.

## The .p8 Private Key

The `.p8` file contains an ECDSA P-256 private key in PKCS#8 PEM format:

```
-----BEGIN PRIVATE KEY-----
MIGTAgEAMBMGByqGSM49AgEGCCqGSM49AwEHBHkwdwIBAQQg...
-----END PRIVATE KEY-----
```

Store it in a secure location:
- Local development: `~/.nanowave/keys/AuthKey_<KeyID>.p8` (chmod 600)
- CI/CD: Environment variable or secrets manager

## JWT Token Generation

Every API request requires a JWT token in the `Authorization: Bearer <token>` header.

### Token Structure

**Header:**
```json
{
  "alg": "ES256",
  "kid": "<Key ID>",
  "typ": "JWT"
}
```

**Payload:**
```json
{
  "iss": "<Issuer ID>",
  "iat": 1623085200,
  "exp": 1623086400,
  "aud": "appstoreconnect-v1"
}
```

### Token Rules

- **Algorithm**: ES256 (ECDSA with P-256 curve and SHA-256)
- **Maximum lifetime**: 20 minutes (`exp - iat <= 1200`)
- **Audience**: Always `"appstoreconnect-v1"`
- **Rate limit**: ~25 requests/second per key

### Go Implementation

```go
package signing

import (
	"crypto/ecdsa"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// APIKeyConfig holds App Store Connect API key credentials.
type APIKeyConfig struct {
	KeyID      string // From App Store Connect (e.g., "ABC123DEFG")
	IssuerID   string // From App Store Connect (UUID format)
	PrivateKey string // Path to .p8 file
}

// TokenProvider generates and caches JWT tokens for the App Store Connect API.
type TokenProvider struct {
	config     APIKeyConfig
	privateKey *ecdsa.PrivateKey

	mu          sync.Mutex
	cachedToken string
	expiresAt   time.Time
}

// NewTokenProvider creates a token provider from an API key config.
func NewTokenProvider(config APIKeyConfig) (*TokenProvider, error) {
	keyData, err := os.ReadFile(config.PrivateKey)
	if err != nil {
		return nil, fmt.Errorf("failed to read private key: %w", err)
	}

	block, _ := pem.Decode(keyData)
	if block == nil {
		return nil, fmt.Errorf("failed to decode PEM block from %s", config.PrivateKey)
	}

	key, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("failed to parse private key: %w", err)
	}

	ecKey, ok := key.(*ecdsa.PrivateKey)
	if !ok {
		return nil, fmt.Errorf("private key is not ECDSA")
	}

	return &TokenProvider{
		config:     config,
		privateKey: ecKey,
	}, nil
}

// Token returns a valid JWT token, generating a new one if the cached token
// is expired or about to expire (within 60 seconds).
func (tp *TokenProvider) Token() (string, error) {
	tp.mu.Lock()
	defer tp.mu.Unlock()

	// Return cached token if still valid (with 60s buffer)
	if tp.cachedToken != "" && time.Now().Add(60*time.Second).Before(tp.expiresAt) {
		return tp.cachedToken, nil
	}

	now := time.Now()
	exp := now.Add(15 * time.Minute) // 15 min (under the 20 min max)

	token := jwt.NewWithClaims(jwt.SigningMethodES256, jwt.MapClaims{
		"iss": tp.config.IssuerID,
		"iat": now.Unix(),
		"exp": exp.Unix(),
		"aud": "appstoreconnect-v1",
	})
	token.Header["kid"] = tp.config.KeyID

	signed, err := token.SignedString(tp.privateKey)
	if err != nil {
		return "", fmt.Errorf("failed to sign JWT: %w", err)
	}

	tp.cachedToken = signed
	tp.expiresAt = exp

	return signed, nil
}
```

### Making API Requests

```go
package signing

import (
	"fmt"
	"io"
	"net/http"
)

const baseURL = "https://api.appstoreconnect.apple.com/v1"

// Client wraps HTTP calls to the App Store Connect API.
type Client struct {
	tokenProvider *TokenProvider
	httpClient    *http.Client
}

// NewClient creates an API client.
func NewClient(tp *TokenProvider) *Client {
	return &Client{
		tokenProvider: tp,
		httpClient:    &http.Client{},
	}
}

// Do executes an authenticated API request.
func (c *Client) Do(method, path string, body io.Reader) (*http.Response, error) {
	token, err := c.tokenProvider.Token()
	if err != nil {
		return nil, fmt.Errorf("failed to get token: %w", err)
	}

	req, err := http.NewRequest(method, baseURL+path, body)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")

	return c.httpClient.Do(req)
}
```

## Required API Key Roles

| Role | Can Do |
|---|---|
| **Admin** | Create certificates, profiles, bundle IDs, register devices |
| **Developer** | Read-only for certificates and profiles |
| **App Manager** | Manage app metadata, no signing access |

For full provisioning automation, **Admin** role is required.

## Security Best Practices

1. **Never commit .p8 files** to version control
2. **Set restrictive permissions**: `chmod 600 AuthKey_*.p8`
3. **Rotate keys periodically** — delete old keys in App Store Connect
4. **Use one key per environment** (development, CI/CD, production)
5. **Store encrypted at rest** when possible (macOS Keychain, secrets manager)
