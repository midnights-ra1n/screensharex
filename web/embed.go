// Package web embeds the built frontend (npm run build) into the server binary.
package web

import "embed"

//go:embed all:dist
var Dist embed.FS
