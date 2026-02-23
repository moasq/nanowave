package orchestration

import (
	"fmt"
	"io/fs"
	"path"
	"regexp"
	"slices"
	"strings"
)

var (
	anthropicSkillNameRE   = regexp.MustCompile(`^[a-z0-9-]{1,64}$`)
	markdownLinkTargetRE   = regexp.MustCompile(`\[[^\]]+\]\(([^)]+)\)`)
	contentsHeadingLineRE  = regexp.MustCompile(`(?i)^\s{0,3}#{1,6}\s+contents\s*$`)
	tableOfContentsLineRE  = regexp.MustCompile(`(?i)table of contents`)
	sourceSkillCategories  = []string{"always", "always-watchos", "always-tvos", "features", "ui", "extensions", "watchos", "tvos", "phases"}
	reservedSkillNameWords = []string{"anthropic", "claude"}
)

const (
	maxSkillDescriptionChars = 1024
	maxSkillBodyLines        = 499 // strict mode: body must be <500 lines
	longReferenceLineLimit   = 100
	tocSearchLineLimit       = 30
)

type SourceSkillComplianceIssue struct {
	Path    string
	Code    string
	Message string
}

func (i SourceSkillComplianceIssue) String() string {
	if i.Path == "" {
		return fmt.Sprintf("%s: %s", i.Code, i.Message)
	}
	return fmt.Sprintf("%s: %s (%s)", i.Code, i.Message, i.Path)
}

type SourceSkillComplianceReport struct {
	Issues                []SourceSkillComplianceIssue
	Notes                 []string
	SkillsChecked         int
	ReferenceFilesChecked int
}

func (r SourceSkillComplianceReport) HasIssues() bool {
	return len(r.Issues) > 0
}

// ValidateAnthropicSourceSkills validates the repository source skill tree under root (e.g. "skills").
// It enforces Anthropic-format skill requirements for skill categories only, and intentionally excludes root/core.
func ValidateAnthropicSourceSkills(fsys fs.FS, root string, strictBestPractices bool) (SourceSkillComplianceReport, error) {
	report := SourceSkillComplianceReport{
		Notes: []string{
			fmt.Sprintf("core rules intentionally excluded from Anthropic skill schema: %s", slashJoin(root, "core")),
		},
	}

	for _, cat := range sourceSkillCategories {
		catPath := slashJoin(root, cat)
		entries, err := fs.ReadDir(fsys, catPath)
		if err != nil {
			report.Issues = append(report.Issues, SourceSkillComplianceIssue{
				Path:    catPath,
				Code:    "missing_skill_category_dir",
				Message: "required skill category directory is missing or unreadable",
			})
			continue
		}

		for _, entry := range entries {
			entryPath := slashJoin(catPath, entry.Name())
			if !entry.IsDir() {
				report.Issues = append(report.Issues, SourceSkillComplianceIssue{
					Path:    entryPath,
					Code:    "non_directory_skill_entry",
					Message: "skill category entries must be directories containing SKILL.md",
				})
				continue
			}

			validateSourceSkillDir(fsys, entryPath, strictBestPractices, &report)
		}
	}

	return report, nil
}

func validateSourceSkillDir(fsys fs.FS, skillDir string, strict bool, report *SourceSkillComplianceReport) {
	skillPath := slashJoin(skillDir, "SKILL.md")
	data, err := fs.ReadFile(fsys, skillPath)
	if err != nil {
		report.Issues = append(report.Issues, SourceSkillComplianceIssue{
			Path:    skillDir,
			Code:    "missing_skill_md",
			Message: "skill directory must contain SKILL.md",
		})
		return
	}
	if len(data) == 0 {
		report.Issues = append(report.Issues, SourceSkillComplianceIssue{
			Path:    skillPath,
			Code:    "empty_skill_md",
			Message: "SKILL.md is empty",
		})
		return
	}

	frontmatter, body, ok := parseMarkdownFrontmatter(string(data))
	if !ok {
		report.Issues = append(report.Issues, SourceSkillComplianceIssue{
			Path:    skillPath,
			Code:    "malformed_frontmatter",
			Message: "SKILL.md must start with YAML frontmatter delimited by ---",
		})
		return
	}

	report.SkillsChecked++

	validateSkillFrontmatter(frontmatter, skillPath, report)
	validateSkillBody(body, skillPath, strict, report)
	validateLocalLinksInFile(fsys, skillDir, skillPath, string(data), false, report)
	validateSkillReferences(fsys, skillDir, string(data), strict, report)
}

func validateSkillFrontmatter(frontmatter map[string]string, skillPath string, report *SourceSkillComplianceReport) {
	if _, ok := frontmatter["name"]; !ok {
		report.Issues = append(report.Issues, SourceSkillComplianceIssue{
			Path:    skillPath,
			Code:    "missing_frontmatter_name",
			Message: "SKILL.md frontmatter must include name",
		})
	}
	if _, ok := frontmatter["description"]; !ok {
		report.Issues = append(report.Issues, SourceSkillComplianceIssue{
			Path:    skillPath,
			Code:    "missing_frontmatter_description",
			Message: "SKILL.md frontmatter must include description",
		})
	}

	for key := range frontmatter {
		if key == "name" || key == "description" {
			continue
		}
		report.Issues = append(report.Issues, SourceSkillComplianceIssue{
			Path:    skillPath,
			Code:    "unsupported_frontmatter_field",
			Message: fmt.Sprintf("unsupported frontmatter field %q (Anthropic skills use only name + description)", key),
		})
	}

	name := frontmatter["name"]
	if name != "" {
		if !anthropicSkillNameRE.MatchString(name) {
			report.Issues = append(report.Issues, SourceSkillComplianceIssue{
				Path:    skillPath,
				Code:    "invalid_skill_name",
				Message: "skill name must match ^[a-z0-9-]{1,64}$",
			})
		}
		for _, reserved := range reservedSkillNameWords {
			if strings.Contains(strings.ToLower(name), reserved) {
				report.Issues = append(report.Issues, SourceSkillComplianceIssue{
					Path:    skillPath,
					Code:    "reserved_skill_name",
					Message: fmt.Sprintf("skill name must not contain reserved term %q", reserved),
				})
				break
			}
		}
	}

	desc := frontmatter["description"]
	if desc == "" {
		report.Issues = append(report.Issues, SourceSkillComplianceIssue{
			Path:    skillPath,
			Code:    "empty_description",
			Message: "skill description must not be empty",
		})
	} else {
		if len(desc) > maxSkillDescriptionChars {
			report.Issues = append(report.Issues, SourceSkillComplianceIssue{
				Path:    skillPath,
				Code:    "description_too_long",
				Message: fmt.Sprintf("skill description must be <= %d chars", maxSkillDescriptionChars),
			})
		}
		if strings.Contains(desc, "<") && strings.Contains(desc, ">") {
			if regexp.MustCompile(`<[^>]+>`).MatchString(desc) {
				report.Issues = append(report.Issues, SourceSkillComplianceIssue{
					Path:    skillPath,
					Code:    "description_contains_markup",
					Message: "skill description must not contain XML/HTML-like tags",
				})
			}
		}
		if !strings.Contains(strings.ToLower(desc), "use when") {
			report.Issues = append(report.Issues, SourceSkillComplianceIssue{
				Path:    skillPath,
				Code:    "description_missing_use_when",
				Message: "skill description must include a 'Use when ...' clause",
			})
		}
	}
}

func validateSkillBody(body, skillPath string, strict bool, report *SourceSkillComplianceReport) {
	if strings.TrimSpace(body) == "" {
		report.Issues = append(report.Issues, SourceSkillComplianceIssue{
			Path:    skillPath,
			Code:    "empty_skill_body",
			Message: "SKILL.md body must not be empty",
		})
		return
	}
	if strict {
		lines := markdownLineCount(body)
		if lines > maxSkillBodyLines {
			report.Issues = append(report.Issues, SourceSkillComplianceIssue{
				Path:    skillPath,
				Code:    "skill_body_too_long",
				Message: "SKILL.md body must be <500 lines in strict mode",
			})
		}
	}
}

func validateSkillReferences(fsys fs.FS, skillDir, skillMarkdown string, strict bool, report *SourceSkillComplianceReport) {
	referenceFiles := referencedMarkdownFilesFromSkill(skillDir, skillMarkdown)
	for _, refPath := range referenceFiles {
		data, err := fs.ReadFile(fsys, refPath)
		if err != nil {
			// Broken link is already reported by validateLocalLinksInFile on SKILL.md.
			continue
		}

		report.ReferenceFilesChecked++
		refText := string(data)

		// Enforce one-level reference loading: reference files should not chain to local markdown files.
		for _, link := range markdownLinkTargets(refText) {
			link = normalizeMarkdownLinkTarget(link)
			if link == "" || isExternalOrAnchorLink(link) {
				continue
			}
			if strings.Contains(link, "\\") {
				report.Issues = append(report.Issues, SourceSkillComplianceIssue{
					Path:    refPath,
					Code:    "local_link_must_use_forward_slashes",
					Message: "local markdown links must use forward slashes",
				})
				continue
			}
			if strings.HasSuffix(strings.ToLower(link), ".md") {
				report.Issues = append(report.Issues, SourceSkillComplianceIssue{
					Path:    refPath,
					Code:    "nested_reference_markdown_link",
					Message: "reference files must not link to other local markdown files (one-level reference rule)",
				})
			}
		}

		if strict && markdownLineCount(refText) > longReferenceLineLimit && !hasTopOfFileTOC(refText) {
			report.Issues = append(report.Issues, SourceSkillComplianceIssue{
				Path:    refPath,
				Code:    "long_reference_missing_toc",
				Message: "reference markdown files >100 lines must include a top-of-file TOC/Contents section",
			})
		}
	}
}

func validateLocalLinksInFile(fsys fs.FS, baseDir, filePath, text string, markdownOnly bool, report *SourceSkillComplianceReport) {
	for _, rawTarget := range markdownLinkTargets(text) {
		target := normalizeMarkdownLinkTarget(rawTarget)
		if target == "" || isExternalOrAnchorLink(target) {
			continue
		}
		if markdownOnly && !strings.HasSuffix(strings.ToLower(target), ".md") {
			continue
		}
		if strings.Contains(target, "\\") {
			report.Issues = append(report.Issues, SourceSkillComplianceIssue{
				Path:    filePath,
				Code:    "local_link_must_use_forward_slashes",
				Message: "local markdown links must use forward slashes",
			})
			continue
		}
		resolved := path.Clean(path.Join(baseDir, target))
		if _, err := fs.Stat(fsys, resolved); err != nil {
			report.Issues = append(report.Issues, SourceSkillComplianceIssue{
				Path:    filePath,
				Code:    "broken_local_link",
				Message: fmt.Sprintf("referenced local path %q does not exist", target),
			})
		}
	}
}

func referencedMarkdownFilesFromSkill(skillDir, skillMarkdown string) []string {
	seen := map[string]bool{}
	var refs []string
	for _, rawTarget := range markdownLinkTargets(skillMarkdown) {
		target := normalizeMarkdownLinkTarget(rawTarget)
		if target == "" || isExternalOrAnchorLink(target) {
			continue
		}
		if strings.Contains(target, "\\") || !strings.HasSuffix(strings.ToLower(target), ".md") {
			continue
		}
		resolved := path.Clean(path.Join(skillDir, target))
		if seen[resolved] {
			continue
		}
		seen[resolved] = true
		refs = append(refs, resolved)
	}
	slices.Sort(refs)
	return refs
}

func markdownLinkTargets(text string) []string {
	matches := markdownLinkTargetRE.FindAllStringSubmatch(text, -1)
	targets := make([]string, 0, len(matches))
	for _, m := range matches {
		if len(m) < 2 {
			continue
		}
		targets = append(targets, m[1])
	}
	return targets
}

func normalizeMarkdownLinkTarget(target string) string {
	target = strings.TrimSpace(target)
	if target == "" {
		return ""
	}
	if i := strings.IndexAny(target, "?#"); i >= 0 {
		target = target[:i]
	}
	return strings.TrimSpace(target)
}

func isExternalOrAnchorLink(target string) bool {
	lower := strings.ToLower(target)
	return strings.HasPrefix(lower, "http://") ||
		strings.HasPrefix(lower, "https://") ||
		strings.HasPrefix(target, "/") ||
		strings.HasPrefix(target, "#")
}

func parseMarkdownFrontmatter(text string) (map[string]string, string, bool) {
	if !strings.HasPrefix(text, "---\n") {
		return nil, text, false
	}
	rest := text[len("---\n"):]
	end := strings.Index(rest, "\n---\n")
	endLen := len("\n---\n")
	if end < 0 {
		// allow EOF immediately after closing fence
		end = strings.Index(rest, "\n---")
		if end < 0 {
			return nil, text, false
		}
		if len(rest) < end+len("\n---") {
			return nil, text, false
		}
		after := rest[end+len("\n---"):]
		if after != "" && after != "\n" {
			return nil, text, false
		}
		endLen = len("\n---")
	}
	fmText := rest[:end]
	body := rest[end+endLen:]
	frontmatter := map[string]string{}
	for _, line := range strings.Split(fmText, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") || !strings.Contains(line, ":") {
			continue
		}
		key, value, _ := strings.Cut(line, ":")
		key = strings.TrimSpace(key)
		value = strings.TrimSpace(value)
		if key == "" {
			continue
		}
		if len(value) >= 2 {
			if (value[0] == '"' && value[len(value)-1] == '"') || (value[0] == '\'' && value[len(value)-1] == '\'') {
				value = value[1 : len(value)-1]
			}
		}
		frontmatter[key] = value
	}
	return frontmatter, body, true
}

func hasTopOfFileTOC(text string) bool {
	lines := strings.Split(text, "\n")
	limit := tocSearchLineLimit
	if len(lines) < limit {
		limit = len(lines)
	}
	for i := 0; i < limit; i++ {
		line := lines[i]
		if contentsHeadingLineRE.MatchString(line) || tableOfContentsLineRE.MatchString(line) {
			return true
		}
	}
	return false
}

func markdownLineCount(text string) int {
	if text == "" {
		return 0
	}
	text = strings.TrimSuffix(text, "\n")
	if text == "" {
		return 0
	}
	return len(strings.Split(text, "\n"))
}

func slashJoin(parts ...string) string {
	var clean []string
	for _, p := range parts {
		if p == "" {
			continue
		}
		clean = append(clean, p)
	}
	if len(clean) == 0 {
		return ""
	}
	return path.Join(clean...)
}
