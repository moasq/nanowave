package claude

import "strings"

// MapModelName preserves the configured model identifier.
func MapModelName(backendModel string) string {
	return strings.TrimSpace(backendModel)
}
