// Package static provides embedded static assets for the web UI.
package static

import (
	"embed"
	"io/fs"
)

//go:embed all:dist
var distFS embed.FS

// Dist returns the embedded web UI assets (index.html, assets/).
// Returns nil if no assets are embedded (development mode).
func Dist() fs.FS {
	// Check if dist directory exists in embedded FS
	if _, err := distFS.ReadDir("dist"); err != nil {
		return nil
	}
	fsys, err := fs.Sub(distFS, "dist")
	if err != nil {
		return nil
	}

	return fsys
}
