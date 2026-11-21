package main

import (
	"fmt"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
)

func compositeOnBg(src *image.NRGBA, bg color.NRGBA) *image.NRGBA {
	b := src.Bounds()
	dst := image.NewNRGBA(b)
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			c := src.NRGBAAt(x, y)
			a := float64(c.A) / 255.0
			r := float64(c.R)*a + float64(bg.R)*(1.0-a)
			g := float64(c.G)*a + float64(bg.G)*(1.0-a)
			blue := float64(c.B)*a + float64(bg.B)*(1.0-a)
			dst.SetNRGBA(x, y, color.NRGBA{
				R: uint8(r + 0.5),
				G: uint8(g + 0.5),
				B: uint8(blue + 0.5),
				A: 255,
			})
		}
	}
	return dst
}

func main() {
	f, _ := os.Open("cmd/genicon/final_test/icon_256.png")
	srcImg, _ := png.Decode(f)
	_ = f.Close()

	nrgba := srcImg.(*image.NRGBA)

	bgs := map[string]color.NRGBA{
		"black":      {R: 0, G: 0, B: 0, A: 255},
		"dark_bar":   {R: 32, G: 32, B: 32, A: 255},
		"light_bar":  {R: 243, G: 243, B: 243, A: 255},
		"white":      {R: 255, G: 255, B: 255, A: 255},
	}

	for name, bg := range bgs {
		comp := compositeOnBg(nrgba, bg)
		out, _ := os.Create(filepath.Join("cmd", "genicon", "final_test", fmt.Sprintf("comp_%s.png", name)))
		_ = png.Encode(out, comp)
		_ = out.Close()
	}
	fmt.Println("Composites generated.")
}
