package ui

import (
	"embed"
	"io/fs"
)

//go:embed assets/*
var embeddedAssets embed.FS

// AssetsFS returns the sub-filesystem rooted at assets/ for serving the embedded UI.
func AssetsFS() (fs.FS, error) {
	return fs.Sub(embeddedAssets, "assets")
}
