//go:build ignore

// gen draws the tray icons: a seated and a standing figure, each in black
// (macOS template image; light Linux bars and Windows taskbars) and white
// (dark bars and taskbars). It also draws unsit.ico, the app icon Windows
// shows on notifications. Run it with `go generate ./assets`.
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

// A paint gives the color at a point of the 32x32 grid, and whether the
// point is opaque. Transparent points keep the color of the nearest shape,
// so a scaled icon gets no dark fringe.
type paint func(x, y float64) (color.NRGBA, bool)

func glyph(shapes []shape, c color.NRGBA) paint {
	return func(x, y float64) (color.NRGBA, bool) {
		for _, s := range shapes {
			if s(x, y) {
				return c, true
			}
		}
		return c, false
	}
}

// badge puts a white glyph on a rounded orange square, legible on light and
// dark notifications alike.
func badge(shapes []shape) paint {
	const k, r = 0.76, 7.0 // the glyph's scale, the corner radius
	figure := glyph(shapes, color.NRGBA{255, 255, 255, 255})
	return func(x, y float64) (color.NRGBA, bool) {
		if c, ok := figure((x-16)/k+16, (y-16)/k+16); ok {
			return c, true
		}
		dx, dy := math.Max(math.Abs(x-16)-(16-r), 0), math.Max(math.Abs(y-16)-(16-r), 0)
		return color.NRGBA{0xD9, 0x48, 0x0F, 255}, math.Hypot(dx, dy) <= r
	}
}

// render draws p at size px with 8x8 supersampling for smooth edges.
func render(p paint, size int) *image.NRGBA {
	const ss = 8
	img := image.NewNRGBA(image.Rect(0, 0, size, size))
	scale := 32 / float64(size)
	for py := range size {
		for px := range size {
			hits, r, g, b := 0, 0, 0, 0
			var out color.NRGBA
			for sy := range ss {
				for sx := range ss {
					x := (float64(px) + (float64(sx)+0.5)/ss) * scale
					y := (float64(py) + (float64(sy)+0.5)/ss) * scale
					c, ok := p(x, y)
					if ok {
						hits++
						r, g, b = r+int(c.R), g+int(c.G), b+int(c.B)
					} else {
						out = c
					}
				}
			}
			if hits > 0 {
				out = color.NRGBA{uint8(r / hits), uint8(g / hits), uint8(b / hits), 0}
			}
			out.A = uint8(hits * 255 / (ss * ss))
			img.SetNRGBA(px, py, out)
		}
	}
	return img
}

var icoSizes = []int{16, 20, 24, 32, 40, 48, 64}

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
	colors := map[string]color.NRGBA{"black": {0, 0, 0, 255}, "white": {255, 255, 255, 255}}
	for name, shapes := range glyphs {
		for cname, c := range colors {
			base := name + "-" + cname
			write(base+".png", encodePNG(render(glyph(shapes, c), 64)))
			write(base+".ico", encodeICO(renderAll(glyph(shapes, c))))
		}
	}
	write("unsit.ico", encodeICO(renderAll(badge(glyphs["stand"]))))
}

func renderAll(p paint) []*image.NRGBA {
	var imgs []*image.NRGBA
	for _, n := range icoSizes {
		imgs = append(imgs, render(p, n))
	}
	return imgs
}

func write(name string, b []byte) {
	if err := os.WriteFile(name, b, 0o644); err != nil {
		log.Fatal(err)
	}
}
