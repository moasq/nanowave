# Certificate Automation

Signing certificates prove your identity to Apple. Every iOS app must be signed with a valid certificate before it can run on a device or be submitted to the App Store.

## Certificate Types

| API Type | Use Case | Max Per Account |
|---|---|---|
| `IOS_DEVELOPMENT` | Run on registered test devices | 2 |
| `IOS_DISTRIBUTION` | App Store & TestFlight | 3 |
| `DEVELOPMENT` | Mac + iOS development (universal) | 2 |
| `DISTRIBUTION` | Mac + iOS distribution (universal) | 3 |

For Nanowave, use `IOS_DEVELOPMENT` for device testing and `IOS_DISTRIBUTION` for App Store submission.

## Full Flow

```
1. Generate ECDSA P-256 private key (local)
        │
        ▼
2. Create CSR from private key (local)
        │
        ▼
3. Submit CSR to Apple API → get certificate (API)
        │
        ▼
4. Import private key + certificate into Keychain (local)
```

## Step 1: Generate Private Key + CSR

The CSR (Certificate Signing Request) tells Apple your public key. You keep the private key locally.

```go
package signing

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"os"
)

// CertKeyPair holds a generated private key and its CSR.
type CertKeyPair struct {
	PrivateKey *ecdsa.PrivateKey
	CSR        []byte // DER-encoded CSR
	CSRPEM     []byte // PEM-encoded CSR (for display/storage)
}

// GenerateCSR creates a new ECDSA P-256 private key and a CSR.
func GenerateCSR(commonName, email string) (*CertKeyPair, error) {
	// Generate ECDSA P-256 private key
	privateKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, fmt.Errorf("failed to generate private key: %w", err)
	}

	// Create CSR template
	template := &x509.CertificateRequest{
		Subject: pkix.Name{
			CommonName:   commonName,
			Organization: []string{commonName},
		},
		EmailAddresses:     []string{email},
		SignatureAlgorithm: x509.ECDSAWithSHA256,
	}

	// Generate CSR
	csrDER, err := x509.CreateCertificateRequest(rand.Reader, template, privateKey)
	if err != nil {
		return nil, fmt.Errorf("failed to create CSR: %w", err)
	}

	// PEM-encode the CSR
	csrPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "CERTIFICATE REQUEST",
		Bytes: csrDER,
	})

	return &CertKeyPair{
		PrivateKey: privateKey,
		CSR:        csrDER,
		CSRPEM:     csrPEM,
	}, nil
}

// SavePrivateKey writes the private key to a PEM file.
func SavePrivateKey(key *ecdsa.PrivateKey, path string) error {
	keyBytes, err := x509.MarshalECPrivateKey(key)
	if err != nil {
		return fmt.Errorf("failed to marshal private key: %w", err)
	}

	keyPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "EC PRIVATE KEY",
		Bytes: keyBytes,
	})

	if err := os.WriteFile(path, keyPEM, 0600); err != nil {
		return fmt.Errorf("failed to write private key: %w", err)
	}

	return nil
}
```

## Step 2: Submit CSR to Apple API

```go
package signing

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
)

// CertificateType represents Apple certificate types.
type CertificateType string

const (
	CertTypeDevelopment  CertificateType = "IOS_DEVELOPMENT"
	CertTypeDistribution CertificateType = "IOS_DISTRIBUTION"
)

// CreateCertificateRequest is the API request body.
type CreateCertificateRequest struct {
	Data struct {
		Type       string `json:"type"`
		Attributes struct {
			CertificateType CertificateType `json:"certificateType"`
			CSRContent      string          `json:"csrContent"`
		} `json:"attributes"`
	} `json:"data"`
}

// CertificateResponse is the API response.
type CertificateResponse struct {
	Data struct {
		ID         string `json:"id"`
		Attributes struct {
			Name               string `json:"name"`
			CertificateType    string `json:"certificateType"`
			DisplayName        string `json:"displayName"`
			SerialNumber       string `json:"serialNumber"`
			ExpirationDate     string `json:"expirationDate"`
			CertificateContent string `json:"certificateContent"` // Base64 DER
		} `json:"attributes"`
	} `json:"data"`
}

// CreateCertificate submits a CSR to Apple and returns the signed certificate.
func (c *Client) CreateCertificate(certType CertificateType, csrPEM []byte) (*CertificateResponse, error) {
	reqBody := CreateCertificateRequest{}
	reqBody.Data.Type = "certificates"
	reqBody.Data.Attributes.CertificateType = certType
	reqBody.Data.Attributes.CSRContent = string(csrPEM)

	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	resp, err := c.Do("POST", "/certificates", bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, fmt.Errorf("failed to create certificate: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 201 {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API returned %d: %s", resp.StatusCode, string(body))
	}

	var certResp CertificateResponse
	if err := json.NewDecoder(resp.Body).Decode(&certResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &certResp, nil
}

// SaveCertificate writes the certificate to a .cer file.
func SaveCertificate(certResp *CertificateResponse, path string) error {
	certDER, err := base64.StdEncoding.DecodeString(certResp.Data.Attributes.CertificateContent)
	if err != nil {
		return fmt.Errorf("failed to decode certificate: %w", err)
	}

	return os.WriteFile(path, certDER, 0644)
}
```

## Step 3: List Existing Certificates

Before creating a new certificate, check if a valid one already exists:

```go
// ListCertificatesResponse is the API response for listing certificates.
type ListCertificatesResponse struct {
	Data []struct {
		ID         string `json:"id"`
		Attributes struct {
			Name            string `json:"name"`
			CertificateType string `json:"certificateType"`
			ExpirationDate  string `json:"expirationDate"`
			SerialNumber    string `json:"serialNumber"`
		} `json:"attributes"`
	} `json:"data"`
}

// ListCertificates returns all certificates, optionally filtered by type.
func (c *Client) ListCertificates(certType CertificateType) (*ListCertificatesResponse, error) {
	path := "/certificates"
	if certType != "" {
		path += "?filter[certificateType]=" + string(certType)
	}

	resp, err := c.Do("GET", path, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to list certificates: %w", err)
	}
	defer resp.Body.Close()

	var listResp ListCertificatesResponse
	if err := json.NewDecoder(resp.Body).Decode(&listResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &listResp, nil
}
```

## Step 4: Install in Keychain

After getting the certificate from Apple, import both the private key and certificate into the macOS Keychain. See [keychain.md](keychain.md) for detailed Keychain operations.

```go
package signing

import (
	"fmt"
	"os/exec"
)

// InstallCertificate imports a .cer file and its private key into a keychain.
func InstallCertificate(certPath, privateKeyPath, keychainPath string) error {
	// Import the private key
	if err := exec.Command("security", "import",
		privateKeyPath,
		"-k", keychainPath,
		"-T", "/usr/bin/codesign",
		"-T", "/usr/bin/security",
	).Run(); err != nil {
		return fmt.Errorf("failed to import private key: %w", err)
	}

	// Import the certificate
	if err := exec.Command("security", "import",
		certPath,
		"-k", keychainPath,
		"-T", "/usr/bin/codesign",
		"-T", "/usr/bin/security",
	).Run(); err != nil {
		return fmt.Errorf("failed to import certificate: %w", err)
	}

	return nil
}
```

## Certificate Renewal

- Development certificates expire after **1 year**
- Distribution certificates expire after **1 year**
- Apple does **not** send expiration warnings via API
- Strategy: Check `expirationDate` on each build, regenerate if < 30 days remaining

```go
import "time"

// IsExpiringSoon returns true if the certificate expires within the given duration.
func IsExpiringSoon(expirationDate string, threshold time.Duration) (bool, error) {
	expiry, err := time.Parse("2006-01-02T15:04:05.000-0700", expirationDate)
	if err != nil {
		return false, fmt.Errorf("failed to parse expiration date: %w", err)
	}

	return time.Until(expiry) < threshold, nil
}
```

## Development vs Distribution

| Aspect | Development | Distribution |
|---|---|---|
| **Certificate type** | `IOS_DEVELOPMENT` | `IOS_DISTRIBUTION` |
| **Use case** | Testing on registered devices | App Store, TestFlight |
| **Requires devices?** | Yes (in profile) | No |
| **Max per account** | 2 | 3 |
| **Can debug?** | Yes | No |
