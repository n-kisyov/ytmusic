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

func renderIconVar(size int, wScale float64, redMode string) *image.NRGBA {
	img := image.NewNRGBA(image.Rect(0, 0, size, size))
	cx := float64(size-1) / 2.0
	cy := float64(size-1) / 2.0
	half := float64(size) / 2.0

	margin := math.Max(0.75, float64(size)*0.04)
	circleR := half - margin - 0.5

	w := float64(size) * wScale
	triHalfH := w * (1.0 / math.Sqrt(3.0))

	opticalOffset := float64(size) * (wScale * 0.05)
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

			normY := clamp01((fy - (half - circleR)) / (2.0 * circleR))
			var r, g, b float64
			switch redMode {
			case "solid_bright": // #FF2A2A
				r, g, b = 255.0, 42.0, 42.0
			case "solid_lighter": // #FF3636
				r, g, b = 255.0, 54.0, 54.0
			case "solid_coral": // #FF4040
				r, g, b = 255.0, 64.0, 64.0
			case "grad_vibrant": // top #FF4444 to bottom #E01414
				r = 255.0*(1.0-normY) + 224.0*normY
				g = 68.0*(1.0-normY) + 20.0*normY
				b = 68.0*(1.0-normY) + 20.0*normY
			default:
				r, g, b = 255.0, 40.0, 40.0
			}

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
	outDir := filepath.Join("cmd", "genicon", "test_out2")
	_ = os.MkdirAll(outDir, 0o755)

	scales := []float64{0.29, 0.32, 0.35}
	modes := []string{"solid_bright", "solid_lighter", "grad_vibrant"}

	for _, sc := range scales {
		for _, m := range modes {
			img := renderIconVar(256, sc, m)
			f, _ := os.Create(filepath.Join(outDir, fmt.Sprintf("scale_%.2f_%s.png", sc, m)))
			_ = png.Encode(f, img)
			_ = f.Close()
		}
	}
	fmt.Println("Generated test_out2 variants.")
}
