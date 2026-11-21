package win32

import (
	"image"
	"testing"
)

func TestRenderAppIconImageSizes(t *testing.T) {
	sizes := []int{16, 24, 32, 48, 64, 128, 256}
	for _, sz := range sizes {
		img := RenderAppIconImage(sz)
		if img == nil {
			t.Fatalf("size %d returned nil image", sz)
		}
		if img.Rect != image.Rect(0, 0, sz, sz) {
			t.Fatalf("size %d bounds expected %v, got %v", sz, image.Rect(0, 0, sz, sz), img.Rect)
		}

		// Ensure all outer corners are completely transparent (no black/dark square background).
		corners := [][2]int{
			{0, 0},
			{sz - 1, 0},
			{0, sz - 1},
			{sz - 1, sz - 1},
		}
		for _, pt := range corners {
			c := img.NRGBAAt(pt[0], pt[1])
			if c.A != 0 {
				t.Fatalf("size %d corner (%d,%d) has alpha %d, expected transparent (0)", sz, pt[0], pt[1], c.A)
			}
		}
	}
}

func TestRenderAppIconImageColorsAndGlyph(t *testing.T) {
	sz := 256
	img := RenderAppIconImage(sz)

	// Sample center pixel (play triangle interior): should be white (255, 255, 255, 255).
	centerColor := img.NRGBAAt(sz/2, sz/2)
	if centerColor.R != 255 || centerColor.G != 255 || centerColor.B != 255 || centerColor.A != 255 {
		t.Fatalf("triangle center color expected white (255,255,255,255), got %+v", centerColor)
	}

	// Sample inside the red circle but outside the triangle (e.g., top-center of circle: x=128, y=40).
	redColor := img.NRGBAAt(128, 40)
	if redColor.A != 255 {
		t.Fatalf("red circle interior alpha expected 255, got %d", redColor.A)
	}
	if redColor.R != 255 || redColor.G != 50 || redColor.B != 50 {
		t.Fatalf("red circle interior expected lighter red (255,50,50), got (%d,%d,%d)", redColor.R, redColor.G, redColor.B)
	}

	// Sample outside corner: should be transparent.
	cornerColor := img.NRGBAAt(5, 5)
	if cornerColor.A != 0 {
		t.Fatalf("corner expected transparent alpha 0, got %d", cornerColor.A)
	}
}

func TestClampHelpers(t *testing.T) {
	if got := clamp01(-0.5); got != 0 {
		t.Errorf("clamp01(-0.5) = %v; want 0", got)
	}
	if got := clamp01(1.5); got != 1 {
		t.Errorf("clamp01(1.5) = %v; want 1", got)
	}
	if got := clamp01(0.42); got != 0.42 {
		t.Errorf("clamp01(0.42) = %v; want 0.42", got)
	}

	if got := clampByte(-10); got != 0 {
		t.Errorf("clampByte(-10) = %v; want 0", got)
	}
	if got := clampByte(300); got != 255 {
		t.Errorf("clampByte(300) = %v; want 255", got)
	}
	if got := clampByte(128.4); got != 128 {
		t.Errorf("clampByte(128.4) = %v; want 128", got)
	}
}
