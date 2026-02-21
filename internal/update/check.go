package update

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// Result holds the outcome of an update check.
type Result struct {
	Latest    string // latest version tag (e.g. "0.2.0")
	Current   string // current running version
	UpdateURL string // URL to the release page
}

// NeedsUpdate returns true if the latest version is newer than current.
func (r *Result) NeedsUpdate() bool {
	return r != nil && compareVersions(r.Latest, r.Current) > 0
}

// ghRelease is the minimal GitHub release JSON we care about.
type ghRelease struct {
	TagName string `json:"tag_name"`
	HTMLURL string `json:"html_url"`
}

// Check queries the GitHub API for the latest release of owner/repo and
// compares it with the current version. It returns nil on any error (network
// failure, bad JSON, etc.) so callers can safely ignore update checks.
func Check(owner, repo, currentVersion string) *Result {
	url := fmt.Sprintf("https://api.github.com/repos/%s/%s/releases/latest", owner, repo)

	client := &http.Client{Timeout: 3 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		return nil
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil
	}

	var rel ghRelease
	if err := json.NewDecoder(resp.Body).Decode(&rel); err != nil {
		return nil
	}

	latest := strings.TrimPrefix(rel.TagName, "v")
	current := strings.TrimPrefix(currentVersion, "v")

	return &Result{
		Latest:    latest,
		Current:   current,
		UpdateURL: rel.HTMLURL,
	}
}

// compareVersions compares two semver-ish strings (major.minor.patch).
// Returns >0 if a > b, <0 if a < b, 0 if equal.
func compareVersions(a, b string) int {
	ap := parseVersion(a)
	bp := parseVersion(b)
	for i := 0; i < 3; i++ {
		if ap[i] != bp[i] {
			return ap[i] - bp[i]
		}
	}
	return 0
}

// parseVersion splits "1.2.3" into [1, 2, 3]. Missing parts default to 0.
func parseVersion(v string) [3]int {
	var parts [3]int
	for i, s := range strings.SplitN(v, ".", 3) {
		if i >= 3 {
			break
		}
		n, _ := strconv.Atoi(s)
		parts[i] = n
	}
	return parts
}
