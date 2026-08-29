//go:build ignore

// Generates the PWA icons in internal/httpserver/static/. Run with: go run scripts/genicons.go
// Pure stdlib (image/png), no external tools or fonts needed - the mark is a
// simple lightning-bolt polygon on a rounded dark square, matching the
// banner's accent color.
package main

import (
	"image"
	"image/color"
	"image/png"
	"log"
	"os"
)

var (
	bg     = color.RGBA{0x0b, 0x0f, 0x14, 0xff}
	accent = color.RGBA{0x5e, 0xb0, 0xef, 0xff}
)

// bolt polygon, in unit coordinates (0..1 of the canvas).
var bolt = [][2]float64{
	{0.58, 0.12}, {0.30, 0.56}, {0.46, 0.56}, {0.40, 0.90},
	{0.72, 0.44}, {0.54, 0.44},
}

func main() {
	for _, size := range []int{192, 512} {
		img := generate(size)
		out, err := os.Create("internal/httpserver/static/icon-" + itoa(size) + ".png")
		if err != nil {
			log.Fatal(err)
		}
		if err := png.Encode(out, img); err != nil {
			log.Fatal(err)
		}
		out.Close()
		log.Printf("wrote internal/httpserver/static/icon-%d.png", size)
	}
}

func generate(size int) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, size, size))
	radius := float64(size) * 0.18
	fsize := float64(size)

	for y := 0; y < size; y++ {
		for x := 0; x < size; x++ {
			fx, fy := float64(x), float64(y)
			if !insideRoundedSquare(fx, fy, fsize, radius) {
				continue // leave transparent
			}
			c := color.Color(bg)
			if pointInPolygon(fx/fsize, fy/fsize, bolt) {
				c = accent
			}
			img.Set(x, y, c)
		}
	}
	return img
}

func insideRoundedSquare(x, y, size, r float64) bool {
	if x < 0 || y < 0 || x > size || y > size {
		return false
	}
	var cx, cy float64
	switch {
	case x < r && y < r:
		cx, cy = r, r
	case x > size-r && y < r:
		cx, cy = size-r, r
	case x < r && y > size-r:
		cx, cy = r, size-r
	case x > size-r && y > size-r:
		cx, cy = size-r, size-r
	default:
		return true // not in a corner box; inside the straight edges
	}
	dx, dy := x-cx, y-cy
	return dx*dx+dy*dy <= r*r
}

// pointInPolygon uses the standard ray-casting rule.
func pointInPolygon(x, y float64, poly [][2]float64) bool {
	inside := false
	n := len(poly)
	for i, j := 0, n-1; i < n; j, i = i, i+1 {
		xi, yi := poly[i][0], poly[i][1]
		xj, yj := poly[j][0], poly[j][1]
		if (yi > y) != (yj > y) && x < (xj-xi)*(y-yi)/(yj-yi)+xi {
			inside = !inside
		}
	}
	return inside
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b []byte
	for n > 0 {
		b = append([]byte{byte('0' + n%10)}, b...)
		n /= 10
	}
	return string(b)
}
