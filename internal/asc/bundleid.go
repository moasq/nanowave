package asc

import "encoding/json"

// FindBundleIDResource searches the bundle IDs list JSON for a matching identifier.
func FindBundleIDResource(data []byte, bundleID string) string {
	var env struct {
		Data []struct {
			ID         string `json:"id"`
			Attributes struct {
				Identifier string `json:"identifier"`
			} `json:"attributes"`
		} `json:"data"`
	}
	if json.Unmarshal(data, &env) != nil {
		return ""
	}
	for _, d := range env.Data {
		if d.Attributes.Identifier == bundleID {
			return d.ID
		}
	}
	return ""
}
