// Package assets embeds gym's stylesheet layer. The embedded FS keeps the
// leading "static/" segment so the file server and the ETag map address files
// exactly as they appear on disk.
package assets

import "embed"

//go:embed all:static
var FS embed.FS
