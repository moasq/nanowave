package agentruntime

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
)

func filterEnv(env []string, name string) []string {
	prefix := name + "="
	filtered := make([]string, 0, len(env))
	for _, e := range env {
		if !strings.HasPrefix(e, prefix) {
			filtered = append(filtered, e)
		}
	}
	return filtered
}

func streamNDJSONLines(r io.Reader, onLine func([]byte) error) error {
	br := bufio.NewReader(r)
	for {
		line, err := br.ReadBytes('\n')
		if len(line) > 0 {
			line = bytes.TrimRight(line, "\r\n")
			if len(bytes.TrimSpace(line)) > 0 {
				if onErr := onLine(line); onErr != nil {
					return onErr
				}
			}
		}
		if err != nil {
			if err == io.EOF {
				return nil
			}
			return err
		}
	}
}

func streamTextLines(r io.Reader, onLine func(string) error) error {
	br := bufio.NewReader(r)
	for {
		line, err := br.ReadBytes('\n')
		if len(line) > 0 {
			text := strings.TrimSpace(string(bytes.TrimRight(line, "\r\n")))
			if text != "" {
				if onErr := onLine(text); onErr != nil {
					return onErr
				}
			}
		}
		if err != nil {
			if err == io.EOF {
				return nil
			}
			return err
		}
	}
}

func buildPromptWithSystem(userMessage string, opts GenerateOpts) string {
	var parts []string
	if strings.TrimSpace(opts.SystemPrompt) != "" {
		parts = append(parts, "System instructions:\n"+strings.TrimSpace(opts.SystemPrompt))
	}
	if strings.TrimSpace(opts.AppendSystemPrompt) != "" {
		parts = append(parts, "Additional system instructions:\n"+strings.TrimSpace(opts.AppendSystemPrompt))
	}
	if strings.TrimSpace(userMessage) != "" {
		parts = append(parts, strings.TrimSpace(userMessage))
	}
	if len(opts.Images) > 0 {
		var b strings.Builder
		b.WriteString("Attached images:\n")
		for _, img := range opts.Images {
			b.WriteString("- ")
			b.WriteString(img)
			b.WriteString("\n")
		}
		b.WriteString("\nHow to handle attached images:\n")
		b.WriteString("1. Read each image to see what it contains.\n")
		b.WriteString("2. Determine intent from the user's message:\n")
		b.WriteString("   - DESIGN REFERENCE (\"make it look like this\"): Analyze visually, do NOT copy into the project.\n")
		b.WriteString("   - APP ASSET (\"use this as the icon\", \"add this image\"): Copy into the project using cp, resize with sips if needed.\n")
		b.WriteString("   - If unclear, default to embedding the image as an app asset.\n")
		b.WriteString("3. For asset integration, use nw_get_skills with key \"user-assets\" for step-by-step instructions.")
		parts = append(parts, strings.TrimSpace(b.String()))
	}
	return strings.Join(parts, "\n\n")
}

func parseFinalOutputFile(path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(data))
}

func extractString(v any, keys ...string) string {
	for _, key := range keys {
		if m, ok := v.(map[string]any); ok {
			if raw, ok := m[key]; ok {
				switch t := raw.(type) {
				case string:
					if strings.TrimSpace(t) != "" {
						return t
					}
				case map[string]any:
					if s := extractString(t, "text", "message", "content"); s != "" {
						return s
					}
				case []any:
					for _, item := range t {
						if s := extractString(item, "text", "message", "content"); s != "" {
							return s
						}
					}
				}
			}
		}
	}
	return ""
}

func decodeJSONLine(line []byte) map[string]any {
	var payload map[string]any
	if err := json.Unmarshal(line, &payload); err != nil {
		return nil
	}
	return payload
}

func stringValue(v any) string {
	switch typed := v.(type) {
	case string:
		return strings.TrimSpace(typed)
	case []byte:
		return strings.TrimSpace(string(typed))
	case json.RawMessage:
		return strings.TrimSpace(string(typed))
	case fmt.Stringer:
		return strings.TrimSpace(typed.String())
	case float64:
		return strconv.FormatFloat(typed, 'f', -1, 64)
	case float32:
		return strconv.FormatFloat(float64(typed), 'f', -1, 32)
	case int:
		return strconv.Itoa(typed)
	case int64:
		return strconv.FormatInt(typed, 10)
	case int32:
		return strconv.FormatInt(int64(typed), 10)
	case bool:
		return strconv.FormatBool(typed)
	default:
		return ""
	}
}

func mapValue(v any) map[string]any {
	if m, ok := v.(map[string]any); ok {
		return m
	}
	return nil
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			return value
		}
	}
	return ""
}

func rawJSON(value any) json.RawMessage {
	switch typed := value.(type) {
	case nil:
		return nil
	case json.RawMessage:
		if len(bytes.TrimSpace(typed)) == 0 {
			return nil
		}
		return typed
	case []byte:
		if len(bytes.TrimSpace(typed)) == 0 {
			return nil
		}
		return json.RawMessage(typed)
	case string:
		trimmed := strings.TrimSpace(typed)
		if trimmed == "" {
			return nil
		}
		if json.Valid([]byte(trimmed)) {
			return json.RawMessage(trimmed)
		}
	}

	encoded, err := json.Marshal(value)
	if err != nil || len(bytes.TrimSpace(encoded)) == 0 || string(encoded) == "null" {
		return nil
	}
	return json.RawMessage(encoded)
}

func appendCapturedLine(buf *strings.Builder, line string) {
	line = strings.TrimSpace(line)
	if line == "" {
		return
	}
	if buf.Len() > 0 {
		buf.WriteByte('\n')
	}
	buf.WriteString(line)
}

func floatValue(v any) float64 {
	switch typed := v.(type) {
	case float64:
		return typed
	case float32:
		return float64(typed)
	case int:
		return float64(typed)
	case int64:
		return float64(typed)
	case json.Number:
		f, _ := typed.Float64()
		return f
	case string:
		f, _ := strconv.ParseFloat(strings.TrimSpace(typed), 64)
		return f
	default:
		return 0
	}
}

func intValue(v any) int {
	switch typed := v.(type) {
	case int:
		return typed
	case int64:
		return int(typed)
	case float64:
		return int(typed)
	case float32:
		return int(typed)
	case json.Number:
		i, _ := typed.Int64()
		return int(i)
	case string:
		i, _ := strconv.Atoi(strings.TrimSpace(typed))
		return i
	default:
		return 0
	}
}
