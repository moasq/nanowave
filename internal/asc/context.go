package asc

import (
	"encoding/json"
	"log"
	"os"
	"path/filepath"
)

// SaveAppID persists the ASC app ID into project_config.json so future runs skip matching.
func SaveAppID(projectDir, appID string) {
	configPath := filepath.Join(projectDir, "project_config.json")
	data, err := os.ReadFile(configPath)
	if err != nil {
		return
	}
	var cfg map[string]any
	if json.Unmarshal(data, &cfg) != nil {
		return
	}
	cfg["asc_app_id"] = appID
	updated, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return
	}
	if writeErr := os.WriteFile(configPath, updated, 0o644); writeErr == nil {
		log.Printf("[asc] saved asc_app_id=%s to project_config.json", appID)
	}
}
