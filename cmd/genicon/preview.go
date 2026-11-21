package main

import (
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

// renderIcon renders the new ytmusic icon:
// - Transparent background (no outer black square).
// - Circular red badge filling the icon canvas with smooth subpixel anti-aliasing.
// - Red shade lightened according to style (e.g., solid lighter red or subtle top-to-bottom depth gradient).
// - Enlarged white play triangle, optically centered, with subpixel anti-aliasing.
func renderIcon(size int, style int) *image.NRGBA {
	img := image.NewNRGBA(image.Rect(0, 0, size, size))
	cx := float64(size-1) / 2.0
	cy := float64(size-1) / 2.0
	half := float64(size) / 2.0

	// Margin ensures anti-aliased circle edge fits comfortably inside the square canvas
	// without any edge clipping on any icon size (16 to 256).
	margin := math.Max(0.75, float64(size)*0.04)
	circleR := half - margin - 0.5

	// White play triangle dimensions:
	// Enlarged relative to previous (W: 0.25 -> ~0.38 of size, height: 0.29 -> ~0.44 of size).
	// For an equilateral triangle pointing right:
	// Height = W * 2 / sqrt(3) ~= W * 1.155 (triHalfH = W / sqrt(3) ~= W * 0.577)
	w := float64(size) * 0.38
	triHalfH := w * (1.0 / math.Sqrt(3.0))

	// Optically center the play triangle inside the circle:
	// A right-pointing triangle has its geometric centroid at 1/3 from the base (triLeft + W/3).
	// Slightly offsetting the centroid (+0.015 * size) balances the visual weight of the pointed apex.
	opticalOffset := float64(size) * 0.018
	triLeft := -w/3.0 + opticalOffset
	triRight := triLeft + w
	triHypot := math.Hypot(w, triHalfH)

	for y := 0; y < size; y++ {
		for x := 0; x < size; x++ {
			fx := float64(x) + 0.5
			fy := float64(y) + 0.5

			// Distance from center to current pixel.
			distC := math.Hypot(fx-half, fy-half)

			// Outer boundary of the red circle:
			// Full alpha inside (distC <= circleR - 0.5), smooth anti-aliasing between circleR - 0.5 and circleR + 0.5.
			if distC > circleR+0.5 {
				continue
			}

			circleAlpha := clamp01(circleR + 0.5 - distC)
			if circleAlpha <= 0 {
				continue
			}

			// Red color options:
			var r, g, b float64
			normY := (fy - (half - circleR)) / (2.0 * circleR) // 0 at top of circle, 1 at bottom
			normY = clamp01(normY)

			switch style {
			case 1:
				// Solid lighter vibrant YouTube red (#FF2A2A -> RGB 255, 42, 42)
				r, g, b = 255.0, 42.0, 42.0
			case 2:
				// Subtle vertical depth gradient: lighter vibrant red at top (#FF3E3E), deeper red at bottom (#E61616)
				r = 255.0*(1.0-normY) + 230.0*normY
				g = 62.0*(1.0-normY) + 22.0*normY
				b = 62.0*(1.0-normY) + 22.0*normY
			case 3:
				// Solid bright coral red (#FF3333 -> RGB 255, 51, 51)
				r, g, b = 255.0, 51.0, 51.0
			default:
				// Vibrant modern red (#FF2626)
				r, g, b = 255.0, 38.0, 38.0
			}

			// Distance to triangle:
			px := fx - cx
			py := fy - cy

			// Triangle edges:
			// 1. Left base: x = triLeft. Inward distance: px - triLeft.
			dBase := px - triLeft

			// 2. Diagonal edges from apex (triRight, 0) to (triLeft, ±triHalfH).
			// Inward distance:
			dDiag := (triHalfH*(triRight-px) - w*math.Abs(py)) / triHypot

			// The signed distance inside the triangle is min(dBase, dDiag).
			dIn := math.Min(dBase, dDiag)

			if dIn > -0.5 {
				triAlpha := clamp01(dIn + 0.5)
				// Blend crisp white over the red base.
				r = r*(1.0-triAlpha) + 255.0*triAlpha
				g = g*(1.0-triAlpha) + 255.0*triAlpha
				b = b*(1.0-triAlpha) + 255.0*triAlpha
			}

			finalAlpha := clampByte(circleAlpha * 255.0)
			img.SetNRGBA(x, y, color.NRGBA{
				R: clampByte(r),
				G: clampByte(g),
				B: clampByte(b),
				A: finalAlpha,
			})
		}
	}
	return img
}

func main() {
	outDir := filepath.Join("cmd", "genicon", "test_out")
	_ = os.MkdirAll(outDir, 0o755)

	styles := []int{1, 2, 3}
	sizes := []int{256, 48, 32, 16}

	for _, st := range styles {
		for _, sz := range sizes {
			img := renderIcon(sz, st)
			f, err := os.Create(filepath.Join(outDir, filepath.Clean(os.Args[0]+"_")+"style"+string(rune('0'+st))+"_"+string(rune('0'+sz/100))+string(rune('0'+(sz%100)/10))+string(rune('0'+sz%10))+".png"))
			if err != nil {
				// simple name
				name := filepath.Join(outDir, "style_"+string(rune('0'+st))+"_"+string(rune('0'+sz/100))+string(rune('0'+(sz%100)/10))+string(rune('0'+sz%10))+".png")
				f, _ = os.Create(name)
			}
			_ = png.Encode(f, img)
			_ = f.Close()
		}
	}
}
