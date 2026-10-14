package static

import "embed"

// FS embeds all CSS, JavaScript, icons, manifest, and service worker files.
//
//go:embed css/* js/* img/* manifest.json sw.js
var FS embed.FS
