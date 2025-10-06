package server

import (
	"embed"
)

//go:embed templates/*.tmpl templates/components/*.tmpl templates/scripts/*.tmpl templates/scripts/api/*.tmpl templates/css/*.tmpl
var TemplateFS embed.FS
