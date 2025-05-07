package server

import (
	"embed"
)

//go:embed templates/*.tmpl templates/components/*.tmpl templates/scripts/*.tmpl
var TemplateFS embed.FS
