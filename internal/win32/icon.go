package win32

import (
	"image"
	"image/color"
	"math"
)

// RenderAppIconImage renders the ytmusic app icon at the requested square pixel dimension:
//   - Transparent background (outer black rounded badge removed).
//   - Circular badge in lighter, vibrant YouTube red (#FF3232) filling the canvas with
//     subpixel anti-aliasing and a comfortable breathing margin to prevent clipping.
//   - Optically centered, enlarged white play triangle with smooth 1-pixel subpixel anti-aliased edges.
func RenderAppIconImage(size int) *image.NRGBA {
	img := image.NewNRGBA(image.Rect(0, 0, size, size))
	cx := float64(size-1) / 2.0
	cy := float64(size-1) / 2.0
	half := float64(size) / 2.0

	// Breathing margin ensures the anti-aliased circle edge does not clip against the square canvas.
	margin := math.Max(0.75, float64(size)*0.04)
	circleR := half - margin - 0.5

	// Slightly enlarged white play triangle (~28% increase in linear dimensions: 0.25 -> 0.32 of size).
	// For an equilateral triangle pointing right:
	//   Height = W * 2 / sqrt(3) ~= W * 1.1547 (triHalfH = W / sqrt(3) ~= W * 0.5774)
	w := float64(size) * 0.32
	triHalfH := w * (1.0 / math.Sqrt(3.0))

	// Optically center the play triangle inside the circle:
	// A right-pointing triangle has its geometric centroid at 1/3 from the left base (triLeft + w/3).
	// A slight forward offset (+0.016 * size) balances the visual weight of the pointed apex.
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

			// Distance inside play triangle:
			// 1. Left base: x = triLeft. Inward distance: px - triLeft.
			// 2. Diagonal edges: apex (triRight, 0) to (triLeft, ±triHalfH).
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
