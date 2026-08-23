package static

import "embed"

// FS embeds all CSS, JavaScript, and asset files.
//
//go:embed css/* js/*
var FS embed.FS
