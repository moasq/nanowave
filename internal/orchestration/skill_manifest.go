package orchestration

import (
	"encoding/json"
	"io/fs"
	"path"
	"sort"
	"strings"
)

// SkillManifest is a machine-readable index of all embedded skills.
// Any LLM provider can consume this to discover and select relevant skills
// without needing to understand nanowave's internal routing.
type SkillManifest struct {
	Version string          `json:"version"`
	Skills  []SkillMetadata `json:"skills"`
}

// SkillMetadata describes one skill for provider-agnostic discovery.
type SkillMetadata struct {
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Category    string   `json:"category"`              // "core", "always", "feature", "ui", "extension", "phase", "platform"
	Tags        []string `json:"tags,omitempty"`         // routing tags: ["swiftui", "navigation", "ios"]
	Platforms   []string `json:"platforms,omitempty"`     // platform filter: ["ios", "watchos", "macos"]
	Path        string   `json:"path"`                   // embedded FS path
	Companions  []string `json:"companions,omitempty"`    // additional reference files
	BodyLines   int      `json:"body_lines"`             // content size hint
	AlwaysLoad  bool     `json:"always_load"`            // true for core/always skills
}

// GenerateSkillManifest scans the embedded skill FS and produces a manifest.
func GenerateSkillManifest() (*SkillManifest, error) {
	manifest := &SkillManifest{
		Version: "1",
	}

	// Scan core rules (always loaded, not Anthropic-format skills)
	coreEntries, err := fs.ReadDir(skillsFS, "skills/core")
	if err == nil {
		for _, e := range coreEntries {
			if e.IsDir() || !strings.HasSuffix(e.Name(), ".md") {
				continue
			}
			data, err := skillsFS.ReadFile("skills/core/" + e.Name())
			if err != nil {
				continue
			}
			desc, _ := extractFrontmatter(string(data))
			name := strings.TrimSuffix(e.Name(), ".md")
			manifest.Skills = append(manifest.Skills, SkillMetadata{
				Name:        name,
				Description: desc,
				Category:    "core",
				Path:        "skills/core/" + e.Name(),
				BodyLines:   markdownLineCount(string(data)),
				AlwaysLoad:  true,
			})
		}
	}

	// Scan category-based skills
	type categoryInfo struct {
		dir        string
		category   string
		alwaysLoad bool
	}
	categories := []categoryInfo{
		{"skills/always", "always", true},
		{"skills/always-watchos", "platform", true},
		{"skills/always-tvos", "platform", true},
		{"skills/always-visionos", "platform", true},
		{"skills/always-macos", "platform", true},
		{"skills/features", "feature", false},
		{"skills/ui", "ui", false},
		{"skills/extensions", "extension", false},
		{"skills/watchos", "platform", false},
		{"skills/tvos", "platform", false},
		{"skills/visionos", "platform", false},
		{"skills/macos", "platform", false},
		{"skills/phases", "phase", false},
	}

	for _, cat := range categories {
		entries, err := fs.ReadDir(skillsFS, cat.dir)
		if err != nil {
			continue
		}
		for _, entry := range entries {
			if !entry.IsDir() {
				continue
			}
			skillDir := cat.dir + "/" + entry.Name()
			skillPath := skillDir + "/SKILL.md"
			data, err := skillsFS.ReadFile(skillPath)
			if err != nil {
				continue
			}

			fm, body, ok := parseMarkdownFrontmatter(string(data))
			if !ok {
				continue
			}

			meta := SkillMetadata{
				Name:        fm["name"],
				Description: fm["description"],
				Category:    cat.category,
				Path:        skillDir,
				BodyLines:   markdownLineCount(body),
				AlwaysLoad:  cat.alwaysLoad,
			}

			// Parse optional metadata
			if fm["category"] != "" {
				meta.Category = fm["category"]
			}
			if tags := fm["tags"]; tags != "" {
				meta.Tags = parseCommaSeparated(tags)
			}
			if platforms := fm["platforms"]; platforms != "" {
				meta.Platforms = parseCommaSeparated(platforms)
			}

			// Detect platform from directory
			if meta.Platforms == nil {
				meta.Platforms = inferPlatforms(cat.dir)
			}

			// Find companion files
			meta.Companions = findCompanionFiles(skillDir)

			manifest.Skills = append(manifest.Skills, meta)
		}
	}

	sort.Slice(manifest.Skills, func(i, j int) bool {
		if manifest.Skills[i].Category != manifest.Skills[j].Category {
			return manifest.Skills[i].Category < manifest.Skills[j].Category
		}
		return manifest.Skills[i].Name < manifest.Skills[j].Name
	})

	return manifest, nil
}

// MarshalManifestJSON serializes the manifest to indented JSON.
func (m *SkillManifest) MarshalManifestJSON() ([]byte, error) {
	return json.MarshalIndent(m, "", "  ")
}

// SkillsForContext returns skills matching the given tags, platform, and feature keys.
// This is the provider-agnostic routing function: any LLM can call this with
// structured context to get the right skills without understanding nanowave internals.
func (m *SkillManifest) SkillsForContext(platform string, featureKeys []string) []SkillMetadata {
	featureSet := make(map[string]bool, len(featureKeys))
	for _, k := range featureKeys {
		featureSet[k] = true
	}

	var result []SkillMetadata
	for _, skill := range m.Skills {
		// Always include always-loaded skills
		if skill.AlwaysLoad {
			// Check platform compatibility
			if len(skill.Platforms) > 0 && platform != "" {
				if !containsIgnoreCase(skill.Platforms, platform) {
					continue
				}
			}
			result = append(result, skill)
			continue
		}

		// Include if skill name matches a feature key
		if featureSet[skill.Name] {
			result = append(result, skill)
			continue
		}

		// Include if any tag matches a feature key
		for _, tag := range skill.Tags {
			if featureSet[tag] {
				result = append(result, skill)
				break
			}
		}
	}
	return result
}

func parseCommaSeparated(s string) []string {
	// Handle both "tag1, tag2" and "tag1,tag2" and YAML list syntax "[tag1, tag2]"
	s = strings.Trim(s, "[]")
	parts := strings.Split(s, ",")
	var result []string
	for _, p := range parts {
		p = strings.TrimSpace(p)
		p = strings.Trim(p, "\"'")
		if p != "" {
			result = append(result, p)
		}
	}
	return result
}

func inferPlatforms(dirPath string) []string {
	switch {
	case strings.Contains(dirPath, "watchos"):
		return []string{"watchos"}
	case strings.Contains(dirPath, "tvos"):
		return []string{"tvos"}
	case strings.Contains(dirPath, "visionos"):
		return []string{"visionos"}
	case strings.Contains(dirPath, "macos"):
		return []string{"macos"}
	default:
		return nil // all platforms
	}
}

func findCompanionFiles(skillDir string) []string {
	var companions []string
	_ = fs.WalkDir(skillsFS, skillDir, func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		if d.Name() == "SKILL.md" {
			return nil
		}
		if strings.HasSuffix(d.Name(), ".md") {
			rel := strings.TrimPrefix(p, skillDir+"/")
			companions = append(companions, rel)
		}
		return nil
	})
	sort.Strings(companions)
	return companions
}

func containsIgnoreCase(slice []string, target string) bool {
	lower := strings.ToLower(target)
	for _, s := range slice {
		if strings.ToLower(s) == lower {
			return true
		}
	}
	return false
}

// FormatSkillsForLLM renders selected skills into a single prompt-ready string.
// This is the universal injection point: any LLM provider calls this to get
// skill content formatted for inclusion in system/user prompts.
func FormatSkillsForLLM(skills []SkillMetadata) string {
	var sb strings.Builder
	for i, skill := range skills {
		data, err := skillsFS.ReadFile(path.Join(skill.Path, "SKILL.md"))
		if err != nil {
			// Try as direct file path (for core rules)
			data, err = skillsFS.ReadFile(skill.Path)
			if err != nil {
				continue
			}
		}
		_, body := extractFrontmatter(string(data))
		if body == "" {
			continue
		}
		if i > 0 {
			sb.WriteString("\n\n---\n\n")
		}
		sb.WriteString(body)

		// Also include companion files
		for _, companion := range skill.Companions {
			companionPath := path.Join(skill.Path, companion)
			cData, err := skillsFS.ReadFile(companionPath)
			if err != nil {
				continue
			}
			_, cBody := extractFrontmatter(string(cData))
			if cBody != "" {
				sb.WriteString("\n\n")
				sb.WriteString(cBody)
			}
		}
	}
	return sb.String()
}
