package main

import (
	"fmt"
	"image"
	"image/color"
	"image/png"
	"math"
	"os"
	"path/filepath"
)

func clamp01(v float64) float64 {
	if v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}
	return v
}

func clampByte(v float64) uint8 {
	if v < 0 {
		return 0
	}
	if v > 255 {
		return 255
	}
	return uint8(v)
}

func renderFinalIcon(size int) *image.NRGBA {
	img := image.NewNRGBA(image.Rect(0, 0, size, size))
	cx := float64(size-1) / 2.0
	cy := float64(size-1) / 2.0
	half := float64(size) / 2.0

	// Margin ensures the anti-aliased circular edge does not clip against the icon boundary.
	margin := math.Max(0.75, float64(size)*0.04)
	circleR := half - margin - 0.5

	// Slightly enlarged white play triangle (~28% increase in linear dimensions: 0.25 -> 0.32).
	wScale := 0.32
	w := float64(size) * wScale
	triHalfH := w * (1.0 / math.Sqrt(3.0))

	// Optical centering: triangle centroid is at 1/3 from left base; offset slightly to balance apex weight.
	opticalOffset := float64(size) * 0.016
	triLeft := -w/3.0 + opticalOffset
	triRight := triLeft + w
	triHypot := math.Hypot(w, triHalfH)

	for y := 0; y < size; y++ {
		for x := 0; x < size; x++ {
			fx := float64(x) + 0.5
			fy := float64(y) + 0.5

			distC := math.Hypot(fx-half, fy-half)
			if distC > circleR+0.5 {
				continue
			}

			circleAlpha := clamp01(circleR + 0.5 - distC)
			if circleAlpha <= 0 {
				continue
			}

			// Lighter, vibrant YouTube red (#FF3232 -> RGB 255, 50, 50).
			r, g, b := 255.0, 50.0, 50.0

			// Signed distance to play triangle.
			px := fx - cx
			py := fy - cy
			dBase := px - triLeft
			dDiag := (triHalfH*(triRight-px) - w*math.Abs(py)) / triHypot
			dIn := math.Min(dBase, dDiag)

			if dIn > -0.5 {
				triAlpha := clamp01(dIn + 0.5)
				r = r*(1.0-triAlpha) + 255.0*triAlpha
				g = g*(1.0-triAlpha) + 255.0*triAlpha
				b = b*(1.0-triAlpha) + 255.0*triAlpha
			}

			img.SetNRGBA(x, y, color.NRGBA{
				R: clampByte(r),
				G: clampByte(g),
				B: clampByte(b),
				A: clampByte(circleAlpha * 255.0),
			})
		}
	}
	return img
}

func main() {
	outDir := filepath.Join("cmd", "genicon", "final_test")
	_ = os.MkdirAll(outDir, 0o755)

	sizes := []int{16, 24, 32, 48, 64, 128, 256}
	for _, sz := range sizes {
		img := renderFinalIcon(sz)
		f, _ := os.Create(filepath.Join(outDir, fmt.Sprintf("icon_%03d.png", sz)))
		_ = png.Encode(f, img)
		_ = f.Close()
	}
	fmt.Println("Rendered all sizes to final_test.")
}
