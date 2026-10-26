// Package assets holds the tray icons drawn by gen.go: a seated and a
// standing figure, black and white, as 64px PNG and multi-size ICO.
package assets

import _ "embed"

//go:generate go run gen.go

var (
	//go:embed sit-black.png
	SitBlackPNG []byte
	//go:embed sit-white.png
	SitWhitePNG []byte
	//go:embed stand-black.png
	StandBlackPNG []byte
	//go:embed stand-white.png
	StandWhitePNG []byte

	//go:embed sit-black.ico
	SitBlackICO []byte
	//go:embed sit-white.ico
	SitWhiteICO []byte
	//go:embed stand-black.ico
	StandBlackICO []byte
	//go:embed stand-white.ico
	StandWhiteICO []byte
)
