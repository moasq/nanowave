package appleauth

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type irisCookieFile struct {
	Version int                          `json:"version"`
	Updated string                       `json:"updated"`
	Cookies map[string][]irisCookieEntry `json:"cookies"`
}

type irisCookieEntry struct {
	Name     string  `json:"name"`
	Value    string  `json:"value"`
	Domain   string  `json:"domain"`
	Path     string  `json:"path"`
	Expires  float64 `json:"expires,omitempty"`
	Secure   bool    `json:"secure"`
	HTTPOnly bool    `json:"httpOnly"`
}

func irisSessionPath(appleID string) string {
	hash := sha256.Sum256([]byte(strings.ToLower(appleID)))
	name := fmt.Sprintf("session-%x.json", hash)
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".asc", "iris", name)
}

// SaveIrisCookies persists iris session cookies for future use.
func SaveIrisCookies(appleID string, jar http.CookieJar) error {
	domains := []string{
		"https://appstoreconnect.apple.com",
		"https://idmsa.apple.com",
		"https://apple.com",
	}
	cookieMap := make(map[string][]irisCookieEntry)
	for _, domain := range domains {
		u, _ := url.Parse(domain)
		for _, c := range jar.Cookies(u) {
			cookieMap[domain] = append(cookieMap[domain], irisCookieEntry{
				Name:     c.Name,
				Value:    c.Value,
				Domain:   c.Domain,
				Path:     c.Path,
				Secure:   c.Secure,
				HTTPOnly: c.HttpOnly,
			})
		}
	}

	file := irisCookieFile{
		Version: 1,
		Updated: time.Now().Format(time.RFC3339),
		Cookies: cookieMap,
	}

	data, err := json.MarshalIndent(file, "", "  ")
	if err != nil {
		return err
	}

	path := irisSessionPath(appleID)
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o600)
}

// ClearIrisSessions removes all stored iris session files.
func ClearIrisSessions() {
	home, _ := os.UserHomeDir()
	irisDir := filepath.Join(home, ".asc", "iris")
	entries, err := os.ReadDir(irisDir)
	if err != nil {
		return
	}
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".json") {
			os.Remove(filepath.Join(irisDir, e.Name()))
		}
	}
	log.Printf("[appleauth] cleared all iris sessions")
}

// LoadIrisCookies restores saved iris session cookies into a cookie jar.
// Returns the jar or an error if no session is available.
func LoadIrisCookies() (http.CookieJar, error) {
	home, _ := os.UserHomeDir()
	irisDir := filepath.Join(home, ".asc", "iris")

	entries, err := os.ReadDir(irisDir)
	if err != nil {
		return nil, fmt.Errorf("no iris sessions: %w", err)
	}

	// Find the most recent session file
	var bestPath string
	var bestTime time.Time
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		info, infoErr := e.Info()
		if infoErr != nil {
			continue
		}
		if bestPath == "" || info.ModTime().After(bestTime) {
			bestPath = filepath.Join(irisDir, e.Name())
			bestTime = info.ModTime()
		}
	}
	if bestPath == "" {
		return nil, fmt.Errorf("no iris session files found")
	}

	// Reject sessions older than 24 hours
	if time.Since(bestTime) > 24*time.Hour {
		return nil, fmt.Errorf("iris session expired (age: %s)", time.Since(bestTime).Round(time.Minute))
	}

	data, err := os.ReadFile(bestPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read session file: %w", err)
	}

	var file irisCookieFile
	if err := json.Unmarshal(data, &file); err != nil {
		return nil, fmt.Errorf("failed to parse session file: %w", err)
	}

	jar, _ := cookiejar.New(nil)
	for domain, cookies := range file.Cookies {
		u, parseErr := url.Parse(domain)
		if parseErr != nil {
			continue
		}
		httpCookies := make([]*http.Cookie, 0, len(cookies))
		for _, c := range cookies {
			httpCookies = append(httpCookies, &http.Cookie{
				Name:     c.Name,
				Value:    c.Value,
				Domain:   c.Domain,
				Path:     c.Path,
				Secure:   c.Secure,
				HttpOnly: c.HTTPOnly,
			})
		}
		jar.SetCookies(u, httpCookies)
	}

	log.Printf("[appleauth] loaded iris session from %s (age: %s)", filepath.Base(bestPath), time.Since(bestTime).Round(time.Second))
	return jar, nil
}
