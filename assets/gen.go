//go:build ignore

// gen draws the tray icons: a seated and a standing figure, each in black
// (macOS template image; light Linux bars and Windows taskbars) and white
// (dark bars and taskbars). Run it with `go generate ./assets`.
package main

import (
	"bytes"
	"encoding/binary"
	"image"
	"image/color"
	"image/png"
	"log"
	"math"
	"os"
)

// Shapes are laid out on a 32x32 grid, y pointing down.
type shape func(x, y float64) bool

func circle(cx, cy, r float64) shape {
	return func(x, y float64) bool { return math.Hypot(x-cx, y-cy) <= r }
}

// capsule is a line from (ax, ay) to (bx, by) with round ends, r thick on each side.
func capsule(ax, ay, bx, by, r float64) shape {
	return func(x, y float64) bool {
		dx, dy := bx-ax, by-ay
		t := math.Max(0, math.Min(1, ((x-ax)*dx+(y-ay)*dy)/(dx*dx+dy*dy)))
		return math.Hypot(x-(ax+t*dx), y-(ay+t*dy)) <= r
	}
}

var glyphs = map[string][]shape{
	// Side view, facing right: back, thigh and shin make a Z.
	"sit": {
		circle(13, 5.5, 3.6),
		capsule(11.5, 12, 11.5, 19.5, 3.2),
		capsule(11.5, 19.5, 21, 19.5, 3.2),
		capsule(21.5, 19.5, 21.5, 28.5, 2.6),
	},
	// Front view: legs apart, arms down.
	"stand": {
		circle(16, 5, 3.6),
		capsule(16, 11.5, 16, 18, 3.4),
		capsule(14.8, 18, 12.2, 28.6, 2.3),
		capsule(17.2, 18, 19.8, 28.6, 2.3),
		capsule(13, 12, 9.8, 19.5, 1.9),
		capsule(19, 12, 22.2, 19.5, 1.9),
	},
}

// render draws a glyph at size px with 8x8 supersampling for smooth edges.
func render(shapes []shape, size int, c color.Gray) *image.NRGBA {
	const ss = 8
	img := image.NewNRGBA(image.Rect(0, 0, size, size))
	scale := 32 / float64(size)
	for py := range size {
		for px := range size {
			hits := 0
			for sy := range ss {
				for sx := range ss {
					x := (float64(px) + (float64(sx)+0.5)/ss) * scale
					y := (float64(py) + (float64(sy)+0.5)/ss) * scale
					for _, s := range shapes {
						if s(x, y) {
							hits++
							break
						}
					}
				}
			}
			img.SetNRGBA(px, py, color.NRGBA{c.Y, c.Y, c.Y, uint8(hits * 255 / (ss * ss))})
		}
	}
	return img
}

func encodePNG(img image.Image) []byte {
	var b bytes.Buffer
	if err := png.Encode(&b, img); err != nil {
		log.Fatal(err)
	}
	return b.Bytes()
}

// encodeICO stores each size as a 32-bit BMP, which every Windows icon loader reads.
func encodeICO(imgs []*image.NRGBA) []byte {
	var dir, data bytes.Buffer
	le := func(w *bytes.Buffer, v any) { binary.Write(w, binary.LittleEndian, v) }
	le(&dir, [3]uint16{0, 1, uint16(len(imgs))})
	offset := 6 + 16*len(imgs)
	for _, img := range imgs {
		n := img.Rect.Dx()
		maskRow := (n + 31) / 32 * 4
		var bmp bytes.Buffer
		// BITMAPINFOHEADER; height counts the color rows and the AND mask rows.
		le(&bmp, struct {
			Size                  uint32
			W, H                  int32
			Planes, Bits          uint16
			Compression, ImgSize  uint32
			XPPM, YPPM, Used, Imp int32
		}{40, int32(n), int32(2 * n), 1, 32, 0, 0, 0, 0, 0, 0})
		for y := n - 1; y >= 0; y-- { // bottom-up BGRA
			for x := range n {
				c := img.NRGBAAt(x, y)
				bmp.Write([]byte{c.B, c.G, c.R, c.A})
			}
		}
		bmp.Write(make([]byte, maskRow*n)) // all-zero mask: alpha decides
		size := 0
		if n < 256 {
			size = n
		}
		le(&dir, struct {
			W, H, Colors, Reserved uint8
			Planes, Bits           uint16
			Bytes, Offset          uint32
		}{uint8(size), uint8(size), 0, 0, 1, 32, uint32(bmp.Len()), uint32(offset)})
		offset += bmp.Len()
		data.Write(bmp.Bytes())
	}
	return append(dir.Bytes(), data.Bytes()...)
}

func main() {
	colors := map[string]color.Gray{"black": {0}, "white": {255}}
	for name, shapes := range glyphs {
		for cname, c := range colors {
			base := name + "-" + cname
			write(base+".png", encodePNG(render(shapes, 64, c)))
			var imgs []*image.NRGBA
			for _, n := range []int{16, 20, 24, 32, 40, 48, 64} {
				imgs = append(imgs, render(shapes, n, c))
			}
			write(base+".ico", encodeICO(imgs))
		}
	}
}

func write(name string, b []byte) {
	if err := os.WriteFile(name, b, 0o644); err != nil {
		log.Fatal(err)
	}
}
