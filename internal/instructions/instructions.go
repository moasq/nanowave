// Package instructions provides embedded instruction files for all runtimes.
// Instructions are organized in two tiers:
//   - rules/  — always loaded into context (core rules + platform-conditional subdirs)
//   - skills/ — flat, lazy-loaded on demand via nw_get_skills
//
// Any runtime (Claude, Codex, Gemini, OpenCode) can consume these.
package instructions

import (
	"embed"
	"io/fs"
	"sort"
	"strings"
)

//go:embed rules skills
var FS embed.FS

// ReadFile reads a file from the embedded instructions FS.
// Paths are relative to the FS root (e.g. "rules/swift-conventions.md", "skills/camera/SKILL.md").
func ReadFile(path string) ([]byte, error) {
	return FS.ReadFile(path)
}

// ReadDir reads a directory from the embedded instructions FS.
func ReadDir(path string) ([]fs.DirEntry, error) {
	return fs.ReadDir(FS, path)
}

// WalkDir walks the embedded instructions FS.
func WalkDir(root string, fn fs.WalkDirFunc) error {
	return fs.WalkDir(FS, root, fn)
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
		body, found := ReadMarkdownBody(path)
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

// LoadRule reads a single top-level rule by name (e.g. "swift-conventions" → rules/swift-conventions.md).
func LoadRule(name string) (string, bool) {
	return ReadMarkdownBody("rules/" + name + ".md")
}

// LoadPlatformRules reads all rules from rules/{platform}/*.md.
// Returns a map of filename (without .md) → body content.
func LoadPlatformRules(platform string) map[string]string {
	dir := "rules/" + platform
	entries, err := ReadDir(dir)
	if err != nil {
		return nil
	}

	result := make(map[string]string)
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".md") {
			continue
		}
		body, found := ReadMarkdownBody(dir + "/" + entry.Name())
		if found && body != "" {
			key := strings.TrimSuffix(entry.Name(), ".md")
			result[key] = body
		}
	}
	return result
}

// LoadSkillContent reads content for a given skill key from skills/{key}/.
// Supports both single-file skills (SKILL.md) and multi-file skills (SKILL.md + reference/).
func LoadSkillContent(key string) string {
	dirPath := "skills/" + key
	if combined := ReadMarkdownDirBodies(dirPath); combined != "" {
		return combined
	}
	// Fallback: try as a direct .md file in skills/
	if body, found := ReadMarkdownBody("skills/" + key + ".md"); found {
		return body
	}
	return ""
}

// ListRuleNames returns the names of all top-level rules/*.md files (without extension).
func ListRuleNames() []string {
	entries, err := ReadDir("rules")
	if err != nil {
		return nil
	}
	var names []string
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".md") {
			continue
		}
		names = append(names, strings.TrimSuffix(entry.Name(), ".md"))
	}
	sort.Strings(names)
	return names
}

// ListSkillKeys returns the directory names under skills/ (each is a loadable skill key).
func ListSkillKeys() []string {
	entries, err := ReadDir("skills")
	if err != nil {
		return nil
	}
	var keys []string
	for _, entry := range entries {
		if entry.IsDir() {
			keys = append(keys, entry.Name())
		}
	}
	sort.Strings(keys)
	return keys
}
