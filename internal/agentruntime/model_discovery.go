package agentruntime

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

type discoveredModelCatalog struct {
	Default string
	Models  []ModelOption
}

func mergeDiscoveredModelCatalogs(catalogs ...discoveredModelCatalog) discoveredModelCatalog {
	merged := discoveredModelCatalog{}
	modelGroups := make([][]ModelOption, 0, len(catalogs))
	for _, catalog := range catalogs {
		if merged.Default == "" && strings.TrimSpace(catalog.Default) != "" {
			merged.Default = strings.TrimSpace(catalog.Default)
		}
		modelGroups = append(modelGroups, catalog.Models)
	}
	merged.Models = MergeModelOptions(modelGroups...)
	if merged.Default == "" && len(merged.Models) > 0 {
		merged.Default = merged.Models[0].ID
	}
	return merged
}

func discoverJSONModelCatalog(paths ...string) discoveredModelCatalog {
	var catalogs []discoveredModelCatalog
	for _, path := range paths {
		path = strings.TrimSpace(expandUserPath(path))
		if path == "" {
			continue
		}
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		var decoded any
		if err := json.Unmarshal(data, &decoded); err != nil {
			continue
		}
		catalogs = append(catalogs, collectJSONModelCatalog(decoded, "Configured in runtime settings"))
	}
	return mergeDiscoveredModelCatalogs(catalogs...)
}

func collectJSONModelCatalog(node any, description string) discoveredModelCatalog {
	var models []ModelOption
	defaultModel := ""
	var walk func(any)
	walk = func(value any) {
		switch typed := value.(type) {
		case map[string]any:
			keys := make([]string, 0, len(typed))
			for key := range typed {
				keys = append(keys, key)
			}
			sort.Strings(keys)
			for _, key := range keys {
				child := typed[key]
				if text, ok := child.(string); ok {
					if priority := modelSettingPriority(key); priority > 0 {
						modelID := strings.TrimSpace(text)
						if modelID != "" {
							models = append(models, ModelOption{
								ID:          modelID,
								Description: description,
							})
							if defaultModel == "" && priority >= 2 {
								defaultModel = modelID
							}
						}
					}
				}
				walk(child)
			}
		case []any:
			for _, child := range typed {
				walk(child)
			}
		}
	}
	walk(node)
	return discoveredModelCatalog{
		Default: defaultModel,
		Models:  MergeModelOptions(models),
	}
}

func modelSettingPriority(rawKey string) int {
	key := strings.ToLower(strings.TrimSpace(rawKey))
	key = strings.ReplaceAll(key, "-", "")
	key = strings.ReplaceAll(key, "_", "")
	switch key {
	case "defaultmodel", "primarymodel":
		return 3
	case "model":
		return 2
	case "fallbackmodel":
		return 1
	default:
		return 0
	}
}

func existingPaths(paths ...string) []string {
	seen := make(map[string]struct{})
	var filtered []string
	for _, raw := range paths {
		path := strings.TrimSpace(expandUserPath(raw))
		if path == "" {
			continue
		}
		path = filepath.Clean(path)
		if _, ok := seen[path]; ok {
			continue
		}
		if info, err := os.Stat(path); err == nil && !info.IsDir() {
			seen[path] = struct{}{}
			filtered = append(filtered, path)
		}
	}
	return filtered
}
