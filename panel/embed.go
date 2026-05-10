package panel

import "embed"

// EmbeddedDist contains the built management panel distribution files.
//
//go:embed dist
var EmbeddedDist embed.FS
