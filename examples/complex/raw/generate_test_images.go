package main

import (
	"fmt"
	"image"
	"image/color"
	"image/gif"
	"image/jpeg"
	"image/png"
	"os"
	"strings"
)

const (
	width    = 320
	height   = 240
	cx       = 160
	cy       = 120
	radius   = 118 // Slightly under 240px diameter.
	fontSize = 54
	scale    = 6
)

type target struct {
	label string
	file  string
}

func main() {
	targets := []target{
		{label: "SVG", file: "test_svg.svg"},
		{label: "PNG", file: "test_png.png"},
		{label: "JPG", file: "test_jpg.jpg"},
		{label: "GIF", file: "test_gif.gif"},
	}

	for _, t := range targets {
		if err := writeImage(t.file, t.label); err != nil {
			fmt.Fprintf(os.Stderr, "error writing %s: %v\n", t.file, err)
			os.Exit(1)
		}
	}

	fmt.Println("Generated files:")
	for _, t := range targets {
		fmt.Printf(" - %s\n", t.file)
	}
}

func writeSVG(path, label string) error {
	svg := fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<svg xmlns="http://www.w3.org/2000/svg" width="%d" height="%d" viewBox="0 0 %d %d">
  <rect width="100%%" height="100%%" fill="red"/>
  <circle cx="%d" cy="%d" r="%d" fill="black"/>
  <text x="50%%" y="50%%" fill="white" font-size="%d" text-anchor="middle" dominant-baseline="middle" font-family="Arial, Helvetica, sans-serif" font-weight="700">%s</text>
</svg>
`, width, height, width, height, cx, cy, radius, fontSize, label)

	return os.WriteFile(path, []byte(svg), 0o644)
}

func writeImage(path, label string) error {
	if strings.HasSuffix(path, ".svg") {
		return writeSVG(path, label)
	}

	img := image.NewRGBA(image.Rect(0, 0, width, height))
	fillRect(img, color.RGBA{R: 255, A: 255})
	drawFilledCircle(img, cx, cy, radius, color.RGBA{A: 255})
	drawTextCentered(img, label, scale, color.RGBA{R: 255, G: 255, B: 255, A: 255})

	file, err := os.Create(path)
	if err != nil {
		return err
	}
	defer file.Close()

	switch {
	case strings.HasSuffix(path, ".png"):
		return png.Encode(file, img)
	case strings.HasSuffix(path, ".jpg"), strings.HasSuffix(path, ".jpeg"):
		return jpeg.Encode(file, img, &jpeg.Options{Quality: 92})
	case strings.HasSuffix(path, ".gif"):
		return gif.Encode(file, img, nil)
	default:
		return fmt.Errorf("unsupported output format: %s", path)
	}
}

func fillRect(img *image.RGBA, c color.RGBA) {
	b := img.Bounds()
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			img.SetRGBA(x, y, c)
		}
	}
}

func drawFilledCircle(img *image.RGBA, centerX, centerY, r int, c color.RGBA) {
	r2 := r * r
	for y := centerY - r; y <= centerY+r; y++ {
		for x := centerX - r; x <= centerX+r; x++ {
			dx := x - centerX
			dy := y - centerY
			if dx*dx+dy*dy <= r2 && image.Pt(x, y).In(img.Bounds()) {
				img.SetRGBA(x, y, c)
			}
		}
	}
}

func drawTextCentered(img *image.RGBA, text string, scale int, c color.RGBA) {
	text = strings.ToUpper(text)
	glyphW := 5 * scale
	glyphH := 7 * scale
	spacing := scale
	totalW := len(text)*glyphW + (len(text)-1)*spacing
	startX := (width - totalW) / 2
	startY := (height - glyphH) / 2

	x := startX
	for _, r := range text {
		drawGlyph(img, x, startY, r, scale, c)
		x += glyphW + spacing
	}
}

func drawGlyph(img *image.RGBA, x0, y0 int, ch rune, scale int, c color.RGBA) {
	rows, ok := glyphs[ch]
	if !ok {
		rows = glyphs['?']
	}
	for y, row := range rows {
		for x, bit := range row {
			if bit != '1' {
				continue
			}
			fillBlock(img, x0+x*scale, y0+y*scale, scale, c)
		}
	}
}

func fillBlock(img *image.RGBA, x0, y0, size int, c color.RGBA) {
	for y := 0; y < size; y++ {
		for x := 0; x < size; x++ {
			px := x0 + x
			py := y0 + y
			if image.Pt(px, py).In(img.Bounds()) {
				img.SetRGBA(px, py, c)
			}
		}
	}
}

var glyphs = map[rune][]string{
	'?': {"11111", "00001", "00110", "00100", "00000", "00100", "00000"},
	'B': {"11110", "10001", "11110", "10001", "10001", "10001", "11110"},
	'F': {"11111", "10000", "11110", "10000", "10000", "10000", "10000"},
	'G': {"01111", "10000", "10000", "10111", "10001", "10001", "01111"},
	'I': {"11111", "00100", "00100", "00100", "00100", "00100", "11111"},
	'J': {"00111", "00010", "00010", "00010", "10010", "10010", "01100"},
	'M': {"10001", "11011", "10101", "10101", "10001", "10001", "10001"},
	'N': {"10001", "11001", "10101", "10011", "10001", "10001", "10001"},
	'P': {"11110", "10001", "10001", "11110", "10000", "10000", "10000"},
	'S': {"01111", "10000", "10000", "01110", "00001", "00001", "11110"},
	'V': {"10001", "10001", "10001", "10001", "01010", "01010", "00100"},
}
