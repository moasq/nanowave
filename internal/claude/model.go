package claude

import "strings"

// mapModelName converts backend model constants to Claude Code CLI model names.
func MapModelName(backendModel string) string {
	switch {
	case strings.Contains(backendModel, "haiku"):
		return "haiku"
	case strings.Contains(backendModel, "sonnet"):
		return "sonnet"
	case strings.Contains(backendModel, "opus"):
		return "opus"
	default:
		return "sonnet"
	}
}
