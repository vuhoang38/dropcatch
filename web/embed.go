// Package web holds the built dashboard so the Go binary can serve it.
package web

import "embed"

// Dist is the output of `bun run build`. Run `make web` before `go build`.
//
//go:embed all:dist
var Dist embed.FS
