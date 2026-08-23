package templates

import "embed"

// FS embeds all HTML templates for zero-dependency deployment.
//
//go:embed *.html
var FS embed.FS
