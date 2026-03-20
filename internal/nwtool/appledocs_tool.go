package nwtool

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

const appleDocsAPIBase = "https://developer.apple.com/tutorials/data/documentation"

var appleDocsHTTPClient = &http.Client{Timeout: 15 * time.Second}

// registerAppleDocsTool registers Apple Developer Documentation tools for all runtimes.
func registerAppleDocsTool(r *Registry) {
	r.Register(appleDocsGetContentTool())
	r.Register(appleDocsSearchSymbolsTool())
	r.Register(appleDocsGetSampleCodeTool())
	r.Register(appleDocsGetRelatedAPIsTool())
	r.Register(appleDocsGetPlatformCompatibilityTool())
}

// fetchAppleDoc fetches a documentation page from Apple's JSON API.
// path is like "swiftui/navigationsplitview" or "avfoundation/avcapturesession".
func fetchAppleDoc(path string) (json.RawMessage, error) {
	url := fmt.Sprintf("%s/%s.json", appleDocsAPIBase, strings.ToLower(path))
	resp, err := appleDocsHTTPClient.Get(url)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch docs: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == 404 {
		return nil, fmt.Errorf("documentation not found for %q — try a different path or check spelling", path)
	}
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("Apple docs API returned %d for %s", resp.StatusCode, path)
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}
	return json.RawMessage(data), nil
}

// extractDocSummary extracts a readable summary from the raw Apple docs JSON.
func extractDocSummary(raw json.RawMessage) map[string]any {
	var doc struct {
		Abstract              []textFragment            `json:"abstract"`
		PrimaryContentSections []primaryContentSection  `json:"primaryContentSections"`
		TopicSections         []topicSection             `json:"topicSections"`
		RelationshipsSections []relationshipSection      `json:"relationshipsSections"`
		SeeAlsoSections       []topicSection             `json:"seeAlsoSections"`
		Kind                  string                     `json:"kind"`
		Hierarchy             hierarchy                  `json:"hierarchy"`
	}
	json.Unmarshal(raw, &doc)

	result := map[string]any{
		"abstract": renderFragments(doc.Abstract),
		"kind":     doc.Kind,
	}

	// Extract declaration
	for _, section := range doc.PrimaryContentSections {
		if section.Kind == "declarations" {
			for _, decl := range section.Declarations {
				tokens := renderDeclarationTokens(decl.Tokens)
				if tokens != "" {
					result["declaration"] = tokens
					break
				}
			}
		}
		if section.Kind == "parameters" {
			var params []string
			for _, p := range section.Parameters {
				params = append(params, fmt.Sprintf("%s: %s", p.Name, renderContentItems(p.Content)))
			}
			if len(params) > 0 {
				result["parameters"] = params
			}
		}
	}

	// Topics (methods, properties, etc.)
	if len(doc.TopicSections) > 0 {
		var topics []map[string]any
		for _, ts := range doc.TopicSections {
			topics = append(topics, map[string]any{
				"title": ts.Title,
				"count": len(ts.Identifiers),
				"items": truncateStrings(ts.Identifiers, 10),
			})
		}
		result["topics"] = topics
	}

	// Relationships (conforms to, inherits from)
	if len(doc.RelationshipsSections) > 0 {
		var rels []map[string]string
		for _, rs := range doc.RelationshipsSections {
			rels = append(rels, map[string]string{
				"type":  rs.Title,
				"items": strings.Join(truncateStrings(rs.Identifiers, 5), ", "),
			})
		}
		result["relationships"] = rels
	}

	// See also
	if len(doc.SeeAlsoSections) > 0 {
		var seeAlso []string
		for _, sa := range doc.SeeAlsoSections {
			seeAlso = append(seeAlso, sa.Title)
		}
		result["see_also"] = seeAlso
	}

	return result
}

type textFragment struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

type primaryContentSection struct {
	Kind         string        `json:"kind"`
	Declarations []declaration `json:"declarations"`
	Parameters   []parameter   `json:"parameters"`
}

type declaration struct {
	Tokens []declarationToken `json:"tokens"`
}

type declarationToken struct {
	Kind string `json:"kind"`
	Text string `json:"text"`
}

type parameter struct {
	Name    string        `json:"name"`
	Content []contentItem `json:"content"`
}

type contentItem struct {
	Type    string         `json:"type"`
	InlineContent []textFragment `json:"inlineContent"`
}

type topicSection struct {
	Title       string   `json:"title"`
	Identifiers []string `json:"identifiers"`
}

type relationshipSection struct {
	Title       string   `json:"title"`
	Identifiers []string `json:"identifiers"`
}

type hierarchy struct {
	Paths [][]string `json:"paths"`
}

func renderFragments(fragments []textFragment) string {
	var parts []string
	for _, f := range fragments {
		parts = append(parts, f.Text)
	}
	return strings.Join(parts, "")
}

func renderDeclarationTokens(tokens []declarationToken) string {
	var parts []string
	for _, t := range tokens {
		parts = append(parts, t.Text)
	}
	return strings.Join(parts, "")
}

func renderContentItems(items []contentItem) string {
	var parts []string
	for _, item := range items {
		for _, ic := range item.InlineContent {
			parts = append(parts, ic.Text)
		}
	}
	return strings.Join(parts, "")
}

func truncateStrings(items []string, max int) []string {
	if len(items) <= max {
		return items
	}
	return items[:max]
}

// --- nw_apple_docs_get_content ---

func appleDocsGetContentTool() *Tool {
	return &Tool{
		Name:        "nw_apple_docs_get_content",
		Description: "Get Apple Developer Documentation for a specific symbol or framework. Returns the declaration, abstract, parameters, topics, and relationships. Use paths like 'swiftui/navigationsplitview', 'avfoundation/avcapturesession', or just 'swiftui' for framework overview.",
		InputSchema: json.RawMessage(`{
  "type": "object",
  "properties": {
    "path": {"type": "string", "description": "Documentation path: framework/symbol (e.g. 'swiftui/navigationsplitview', 'healthkit/hkhealthstore', 'swiftui')"}
  },
  "required": ["path"]
}`),
		Handler: func(_ context.Context, input json.RawMessage) (json.RawMessage, error) {
			var in struct {
				Path string `json:"path"`
			}
			if err := json.Unmarshal(input, &in); err != nil {
				return jsonError("invalid input: " + err.Error())
			}
			raw, err := fetchAppleDoc(in.Path)
			if err != nil {
				return jsonError(err.Error())
			}
			summary := extractDocSummary(raw)
			summary["path"] = in.Path
			summary["url"] = fmt.Sprintf("https://developer.apple.com/documentation/%s", strings.ToLower(in.Path))
			return jsonOK(summary)
		},
	}
}

// --- nw_apple_docs_search_symbols ---

func appleDocsSearchSymbolsTool() *Tool {
	return &Tool{
		Name:        "nw_apple_docs_search_symbols",
		Description: "Search for symbols within a framework. Lists all available types, methods, and properties. Use the framework path (e.g. 'swiftui', 'avfoundation', 'healthkit').",
		InputSchema: json.RawMessage(`{
  "type": "object",
  "properties": {
    "framework": {"type": "string", "description": "Framework name e.g. 'swiftui', 'avfoundation', 'healthkit', 'realitykit'"}
  },
  "required": ["framework"]
}`),
		Handler: func(_ context.Context, input json.RawMessage) (json.RawMessage, error) {
			var in struct {
				Framework string `json:"framework"`
			}
			if err := json.Unmarshal(input, &in); err != nil {
				return jsonError("invalid input: " + err.Error())
			}
			raw, err := fetchAppleDoc(in.Framework)
			if err != nil {
				return jsonError(err.Error())
			}
			var doc struct {
				TopicSections []topicSection `json:"topicSections"`
			}
			json.Unmarshal(raw, &doc)

			var sections []map[string]any
			for _, ts := range doc.TopicSections {
				// Extract short symbol names from full identifiers
				var names []string
				for _, id := range ts.Identifiers {
					parts := strings.Split(id, "/")
					names = append(names, parts[len(parts)-1])
				}
				sections = append(sections, map[string]any{
					"title":   ts.Title,
					"count":   len(ts.Identifiers),
					"symbols": truncateStrings(names, 20),
				})
			}
			return jsonOK(map[string]any{
				"framework": in.Framework,
				"sections":  sections,
				"url":       fmt.Sprintf("https://developer.apple.com/documentation/%s", strings.ToLower(in.Framework)),
			})
		},
	}
}

// --- nw_apple_docs_get_sample_code ---

func appleDocsGetSampleCodeTool() *Tool {
	return &Tool{
		Name:        "nw_apple_docs_get_sample_code",
		Description: "Get code examples and usage patterns for an Apple API. Returns primary content sections with code listings from the documentation page.",
		InputSchema: json.RawMessage(`{
  "type": "object",
  "properties": {
    "path": {"type": "string", "description": "Documentation path e.g. 'swiftui/navigationsplitview', 'swiftdata/modelcontainer'"}
  },
  "required": ["path"]
}`),
		Handler: func(_ context.Context, input json.RawMessage) (json.RawMessage, error) {
			var in struct {
				Path string `json:"path"`
			}
			if err := json.Unmarshal(input, &in); err != nil {
				return jsonError("invalid input: " + err.Error())
			}
			raw, err := fetchAppleDoc(in.Path)
			if err != nil {
				return jsonError(err.Error())
			}
			var doc struct {
				PrimaryContentSections []json.RawMessage `json:"primaryContentSections"`
				Abstract               []textFragment     `json:"abstract"`
			}
			json.Unmarshal(raw, &doc)

			// Extract code blocks from content sections
			var codeBlocks []string
			for _, sectionRaw := range doc.PrimaryContentSections {
				var section struct {
					Kind    string `json:"kind"`
					Content []struct {
						Type   string `json:"type"`
						Syntax string `json:"syntax"`
						Code   string `json:"code"`
					} `json:"content"`
				}
				json.Unmarshal(sectionRaw, &section)
				if section.Kind == "content" {
					for _, c := range section.Content {
						if c.Type == "codeListing" && c.Code != "" {
							codeBlocks = append(codeBlocks, c.Code)
						}
					}
				}
			}

			return jsonOK(map[string]any{
				"path":        in.Path,
				"abstract":    renderFragments(doc.Abstract),
				"code_blocks": codeBlocks,
				"url":         fmt.Sprintf("https://developer.apple.com/documentation/%s", strings.ToLower(in.Path)),
			})
		},
	}
}

// --- nw_apple_docs_get_related_apis ---

func appleDocsGetRelatedAPIsTool() *Tool {
	return &Tool{
		Name:        "nw_apple_docs_get_related_apis",
		Description: "Get related APIs and 'See Also' suggestions for an Apple API. Helps discover alternative or complementary APIs.",
		InputSchema: json.RawMessage(`{
  "type": "object",
  "properties": {
    "path": {"type": "string", "description": "Documentation path e.g. 'swiftui/list', 'avfoundation/avplayer'"}
  },
  "required": ["path"]
}`),
		Handler: func(_ context.Context, input json.RawMessage) (json.RawMessage, error) {
			var in struct {
				Path string `json:"path"`
			}
			if err := json.Unmarshal(input, &in); err != nil {
				return jsonError("invalid input: " + err.Error())
			}
			raw, err := fetchAppleDoc(in.Path)
			if err != nil {
				return jsonError(err.Error())
			}
			var doc struct {
				SeeAlsoSections       []topicSection      `json:"seeAlsoSections"`
				RelationshipsSections []relationshipSection `json:"relationshipsSections"`
			}
			json.Unmarshal(raw, &doc)

			var seeAlso []map[string]any
			for _, sa := range doc.SeeAlsoSections {
				var names []string
				for _, id := range sa.Identifiers {
					parts := strings.Split(id, "/")
					names = append(names, parts[len(parts)-1])
				}
				seeAlso = append(seeAlso, map[string]any{
					"title":   sa.Title,
					"symbols": names,
				})
			}

			var relationships []map[string]any
			for _, rs := range doc.RelationshipsSections {
				var names []string
				for _, id := range rs.Identifiers {
					parts := strings.Split(id, "/")
					names = append(names, parts[len(parts)-1])
				}
				relationships = append(relationships, map[string]any{
					"type":    rs.Title,
					"symbols": names,
				})
			}

			return jsonOK(map[string]any{
				"path":          in.Path,
				"see_also":      seeAlso,
				"relationships": relationships,
			})
		},
	}
}

// --- nw_apple_docs_get_platform_compatibility ---

func appleDocsGetPlatformCompatibilityTool() *Tool {
	return &Tool{
		Name:        "nw_apple_docs_get_platform_compatibility",
		Description: "Check which platforms and OS versions support a specific Apple API. Returns availability information from the documentation.",
		InputSchema: json.RawMessage(`{
  "type": "object",
  "properties": {
    "path": {"type": "string", "description": "Documentation path e.g. 'swiftui/navigationstack', 'healthkit/hkhealthstore'"}
  },
  "required": ["path"]
}`),
		Handler: func(_ context.Context, input json.RawMessage) (json.RawMessage, error) {
			var in struct {
				Path string `json:"path"`
			}
			if err := json.Unmarshal(input, &in); err != nil {
				return jsonError("invalid input: " + err.Error())
			}
			raw, err := fetchAppleDoc(in.Path)
			if err != nil {
				return jsonError(err.Error())
			}
			var doc struct {
				Variants []struct {
					Paths  []string `json:"paths"`
					Traits []struct {
						InterfaceLanguage string `json:"interfaceLanguage"`
					} `json:"traits"`
				} `json:"variants"`
				Hierarchy hierarchy `json:"hierarchy"`
				Metadata  struct {
					Platforms []struct {
						Name          string `json:"name"`
						IntroducedAt string `json:"introducedAt"`
					} `json:"platforms"`
				} `json:"metadata"`
			}
			json.Unmarshal(raw, &doc)

			var platforms []map[string]string
			for _, p := range doc.Metadata.Platforms {
				platforms = append(platforms, map[string]string{
					"platform":     p.Name,
					"introduced_at": p.IntroducedAt,
				})
			}

			var languages []string
			for _, v := range doc.Variants {
				for _, t := range v.Traits {
					if t.InterfaceLanguage != "" {
						languages = append(languages, t.InterfaceLanguage)
					}
				}
			}

			return jsonOK(map[string]any{
				"path":      in.Path,
				"platforms": platforms,
				"languages": languages,
			})
		},
	}
}
