package win32

import (
	"image"
	"image/color"
	"math"
)

// RenderAppIconImage renders the ytmusic icon (dark obsidian rounded badge, crimson circle,
// and white play glyph) at the requested square pixel dimension.
func RenderAppIconImage(size int) *image.NRGBA {
	img := image.NewNRGBA(image.Rect(0, 0, size, size))
	cx := float64(size-1) / 2.0
	cy := float64(size-1) / 2.0
	half := float64(size) / 2.0
	corner := float64(size) * 0.24
	margin := math.Max(1.0, float64(size)*0.03)
	rimWidth := math.Max(1.3, float64(size)*0.042)

	for y := 0; y < size; y++ {
		for x := 0; x < size; x++ {
			fx := float64(x) + 0.5
			fy := float64(y) + 0.5

			// Signed distance to rounded square.
			dx := math.Abs(fx-half) - (half - corner - margin)
			dy := math.Abs(fy-half) - (half - corner - margin)
			outDist := math.Hypot(math.Max(dx, 0), math.Max(dy, 0)) + math.Min(math.Max(dx, dy), 0) - corner

			if outDist > 0.7 {
				img.SetNRGBA(x, y, color.NRGBA{})
				continue
			}

			// Base dark obsidian background.
			bgAlpha := clampByte((0.7 - outDist) * 255.0)
			r, g, b := 24.0, 24.0, 32.0

			// Outer subtle rim highlight.
			if outDist > -rimWidth {
				r, g, b = 58.0, 58.0, 74.0
			}

			// Crimson circle in center.
			distC := math.Hypot(fx-half, fy-half)
			circleR := float64(size) * 0.34
			if distC < circleR+0.7 {
				mix := clamp01(circleR + 0.7 - distC)
				r = r*(1-mix) + 255.0*mix
				g = g*(1-mix) + 45.0*mix
				b = b*(1-mix) + 85.0*mix
			}

			// Crisp white play triangle inside the circle.
			px := fx - cx
			py := fy - cy
			triLeft := -float64(size) * 0.09
			triRight := float64(size) * 0.16
			triHalfH := float64(size) * 0.145
			if px >= triLeft && px <= triRight {
				t := (px - triLeft) / (triRight - triLeft)
				maxY := triHalfH * (1.0 - t)
				if math.Abs(py) <= maxY {
					r, g, b = 255, 255, 255
				}
			}

			img.SetNRGBA(x, y, color.NRGBA{
				R: clampByte(r),
				G: clampByte(g),
				B: clampByte(b),
				A: bgAlpha,
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
