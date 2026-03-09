package hooks

import "embed"

//go:embed templates/*.yaml
var TemplateFS embed.FS

// TemplateNames maps template identifiers to their embedded file paths.
var TemplateNames = map[string]string{
	"indie": "templates/indie.yaml",
	"team":  "templates/team.yaml",
	"ci":    "templates/ci.yaml",
}

// GetTemplate returns the content of an embedded template by name.
func GetTemplate(name string) ([]byte, error) {
	path, ok := TemplateNames[name]
	if !ok {
		return nil, &TemplateNotFoundError{Name: name}
	}
	return TemplateFS.ReadFile(path)
}

// TemplateNotFoundError is returned when a template name is not recognized.
type TemplateNotFoundError struct {
	Name string
}

func (e *TemplateNotFoundError) Error() string {
	return "unknown template: " + e.Name + " (available: indie, team, ci)"
}
