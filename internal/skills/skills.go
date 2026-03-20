// Package skills provides embedded skill files for all runtimes.
// Skills are markdown files organized by category: core rules, always-on skills,
// conditional features, and platform overrides.
// Any runtime (Claude, Codex, Gemini, OpenCode) can consume these.
package skills

import (
	"embed"
	"fmt"
	"io/fs"
	"strings"
)

//go:embed data
var FS embed.FS

// ReadFile reads a file from the embedded skills FS.
// Paths are relative to the data/ root (e.g. "core/swift-conventions.md").
func ReadFile(path string) ([]byte, error) {
	return FS.ReadFile("data/" + path)
}

// ReadDir reads a directory from the embedded skills FS.
func ReadDir(path string) ([]fs.DirEntry, error) {
	return fs.ReadDir(FS, "data/"+path)
}

// WalkDir walks the embedded skills FS.
func WalkDir(root string, fn fs.WalkDirFunc) error {
	return fs.WalkDir(FS, "data/"+root, fn)
}

// ExtractFrontmatter splits a markdown file into YAML frontmatter description and body.
func ExtractFrontmatter(content string) (description string, body string) {
	if !strings.HasPrefix(content, "---") {
		return "", content
	}
	end := strings.Index(content[3:], "---")
	if end < 0 {
		return "", content
	}
	frontmatter := content[3 : end+3]
	body = strings.TrimSpace(content[end+6:])

	for _, line := range strings.Split(frontmatter, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "description:") {
			desc := strings.TrimPrefix(line, "description:")
			desc = strings.TrimSpace(desc)
			desc = strings.Trim(desc, "\"'")
			return desc, body
		}
	}
	return "", body
}

// ReadMarkdownBody reads a file and returns the body stripped of YAML frontmatter.
func ReadMarkdownBody(path string) (body string, found bool) {
	data, err := ReadFile(path)
	if err != nil {
		return "", false
	}
	_, body = ExtractFrontmatter(string(data))
	return body, true
}

// ReadMarkdownDirBodies combines all markdown bodies from a directory.
func ReadMarkdownDirBodies(dirPath string) string {
	var combined strings.Builder

	if body, found := ReadMarkdownBody(dirPath + "/SKILL.md"); found && body != "" {
		combined.WriteString(body)
	}

	_ = WalkDir(dirPath, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		if !strings.HasSuffix(d.Name(), ".md") || d.Name() == "SKILL.md" {
			return nil
		}
		// Strip the "data/" prefix for ReadMarkdownBody
		relPath := strings.TrimPrefix(path, "data/")
		body, found := ReadMarkdownBody(relPath)
		if !found || body == "" {
			return nil
		}
		if combined.Len() > 0 {
			combined.WriteString("\n\n")
		}
		combined.WriteString(body)
		return nil
	})
	return combined.String()
}

// LoadRuleContent reads content for a given rule_key from the embedded FS.
// Searches core/, always/, features/, ui/, extensions/, and platform dirs for the key.
// Supports platform-prefixed keys like "watchos-biometrics" → watchos/biometrics/.
func LoadRuleContent(ruleKey string) string {
	corePath := fmt.Sprintf("core/%s.md", ruleKey)
	if body, found := ReadMarkdownBody(corePath); found {
		return body
	}

	for _, cat := range []string{"always", "features", "ui", "extensions"} {
		filePath := fmt.Sprintf("%s/%s.md", cat, ruleKey)
		if body, found := ReadMarkdownBody(filePath); found && body != "" {
			return body
		}
		dirPath := fmt.Sprintf("%s/%s", cat, ruleKey)
		if combined := ReadMarkdownDirBodies(dirPath); combined != "" {
			return combined
		}
	}

	// Platform-specific skills: search watchos/, tvos/, visionos/, macos/
	for _, plat := range []string{"watchos", "tvos", "visionos", "macos"} {
		// Direct match: e.g. key="biometrics" in watchos/biometrics/
		dirPath := fmt.Sprintf("%s/%s", plat, ruleKey)
		if combined := ReadMarkdownDirBodies(dirPath); combined != "" {
			return combined
		}
		// Platform-prefixed match: e.g. key="watchos-biometrics" → watchos/biometrics/
		if strings.HasPrefix(ruleKey, plat+"-") {
			subKey := strings.TrimPrefix(ruleKey, plat+"-")
			dirPath := fmt.Sprintf("%s/%s", plat, subKey)
			if combined := ReadMarkdownDirBodies(dirPath); combined != "" {
				return combined
			}
		}
	}

	return ""
}
