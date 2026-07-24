package generator

import (
	"embed"
	"strings"
	"text/template"
)

//go:embed tmpl
var templateFS embed.FS

var templates = template.Must(
	template.New("").ParseFS(templateFS, "tmpl/*.tmpl"),
)

// ExecuteTemplate renders a named template against data and returns the result.
// Panics on error: templates are static and data is validated; any failure is a programmer bug.
func ExecuteTemplate(name string, data any) string {
	var b strings.Builder
	if err := templates.ExecuteTemplate(&b, name, data); err != nil {
		panic("dockerfile: executing " + name + ": " + err.Error())
	}
	return strings.TrimRight(b.String(), "\n")
}
