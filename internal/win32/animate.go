//go:build windows

package win32

import "unsafe"

const (
	timerResizeID   = 1
	timerTrayAnimID = 2

	resizeSteps   = 8
	trayAnimSteps = 11
	animTickMs    = 14

	wsExLayered = 0x00080000
	lwaAlpha    = 0x00000002

	swHide = 0
	swShow = 5

	swpNoSize       = 0x0001
	swpNoMove       = 0x0002
	swpNoZOrder     = 0x0004
	swpNoActivate   = 0x0010
	swpFrameChanged = 0x0020
)

type resizeAnimState struct {
	active  bool
	step    int
	startH  int32
	targetH int32
	winW    int32
}

type trayAnimMode int

const (
	trayAnimNone trayAnimMode = iota
	trayAnimMinimize
	trayAnimRestore
)

type trayAnimState struct {
	mode     trayAnimMode
	step     int
	fromRect rect
	toRect   rect
}

// MinimizeToTray smoothly animates the window toward the system tray icon and hides it.
func (c *Controller) MinimizeToTray() {
	if c.hidden || c.trayAnim.mode == trayAnimMinimize {
		return
	}

	// Cancel any in-progress height animation cleanly.
	if c.resizeAnim.active {
		c.resizeAnim.active = false
		procKillTimer.Call(c.hwnd, timerResizeID)
	}

	var cur rect
	procGetWindowRect.Call(c.hwnd, uintptr(unsafe.Pointer(&cur)))
	if (cur.Right - cur.Left) > 80 && (cur.Bottom - cur.Top) > 80 {
		c.savedRect = cur
	}

	target := c.getTrayIconRect()

	// Enable WS_EX_LAYERED for smooth alpha fade.
	exStyle := getWindowLongPtr(c.hwnd, gwlExStyle)
	if (exStyle & wsExLayered) == 0 {
		setWindowLongPtr(c.hwnd, gwlExStyle, exStyle|wsExLayered)
	}
	procSetLayeredWindowAttributes.Call(c.hwnd, 0, 255, lwaAlpha)

	c.trayAnim = trayAnimState{
		mode:     trayAnimMinimize,
		step:     0,
		fromRect: cur,
		toRect:   target,
	}
	procSetTimer.Call(c.hwnd, timerTrayAnimID, animTickMs, 0)
}

// RestoreFromTray smoothly animates the window back from the system tray and focuses it.
func (c *Controller) RestoreFromTray() {
	if !c.hidden && c.trayAnim.mode != trayAnimMinimize {
		procSetForegroundWindow.Call(c.hwnd)
		return
	}

	if (c.savedRect.Right - c.savedRect.Left) <= 80 {
		var cur rect
		procGetWindowRect.Call(c.hwnd, uintptr(unsafe.Pointer(&cur)))
		c.savedRect = cur
	}

	start := c.getTrayIconRect()
	// Give the starting rect a small minimal footprint near the tray so WebView2 paints cleanly.
	startW := (c.savedRect.Right - c.savedRect.Left) / 3
	startH := (c.savedRect.Bottom - c.savedRect.Top) / 3
	if startW < 100 {
		startW = 100
	}
	if startH < 60 {
		startH = 60
	}
	startRect := rect{
		Left:   start.Left - startW/2,
		Top:    start.Top - startH,
		Right:  start.Left + startW/2,
		Bottom: start.Top,
	}

	exStyle := getWindowLongPtr(c.hwnd, gwlExStyle)
	if (exStyle & wsExLayered) == 0 {
		setWindowLongPtr(c.hwnd, gwlExStyle, exStyle|wsExLayered)
	}
	procSetLayeredWindowAttributes.Call(c.hwnd, 0, 10, lwaAlpha)
	procSetWindowPos.Call(
		c.hwnd, 0,
		uintptr(startRect.Left), uintptr(startRect.Top),
		uintptr(startRect.Right-startRect.Left), uintptr(startRect.Bottom-startRect.Top),
		swpNoZOrder|swpNoActivate,
	)
	procShowWindow.Call(c.hwnd, swShow)

	c.hidden = false
	c.trayAnim = trayAnimState{
		mode:     trayAnimRestore,
		step:     0,
		fromRect: startRect,
		toRect:   c.savedRect,
	}
	procSetTimer.Call(c.hwnd, timerTrayAnimID, animTickMs, 0)
}

// ToggleTrayVisibility restores from tray if hidden, or minimizes to tray if visible.
func (c *Controller) ToggleTrayVisibility() {
	if c.hidden || c.trayAnim.mode == trayAnimMinimize {
		c.RestoreFromTray()
		return
	}
	c.MinimizeToTray()
}

func (c *Controller) stepTrayAnimation() {
	if c.trayAnim.mode == trayAnimNone {
		procKillTimer.Call(c.hwnd, timerTrayAnimID)
		return
	}

	c.trayAnim.step++
	t := float64(c.trayAnim.step) / float64(trayAnimSteps)
	if t > 1 {
		t = 1
	}

	var ease float64
	var alpha uint8
	if c.trayAnim.mode == trayAnimMinimize {
		// Cubic ease-in toward tray.
		ease = t * t * t
		alpha = uint8(255.0 * (1.0 - t*t))
	} else {
		// Cubic ease-out back to saved normal bounds.
		inv := 1.0 - t
		ease = 1.0 - inv*inv*inv
		alpha = uint8(10.0 + 245.0*ease)
	}

	r := lerpRect(c.trayAnim.fromRect, c.trayAnim.toRect, ease)
	w := r.Right - r.Left
	h := r.Bottom - r.Top
	if w < 16 {
		w = 16
	}
	if h < 16 {
		h = 16
	}

	procSetLayeredWindowAttributes.Call(c.hwnd, 0, uintptr(alpha), lwaAlpha)
	procSetWindowPos.Call(c.hwnd, 0, uintptr(r.Left), uintptr(r.Top), uintptr(w), uintptr(h), swpNoZOrder|swpNoActivate)

	if c.trayAnim.step >= trayAnimSteps {
		procKillTimer.Call(c.hwnd, timerTrayAnimID)
		completedMode := c.trayAnim.mode
		c.trayAnim.mode = trayAnimNone

		if completedMode == trayAnimMinimize {
			procShowWindow.Call(c.hwnd, swHide)
			c.hidden = true
			// Restore normal window dimensions while hidden so the next restore starts crisp.
			procSetWindowPos.Call(
				c.hwnd, 0,
				uintptr(c.savedRect.Left), uintptr(c.savedRect.Top),
				uintptr(c.savedRect.Right-c.savedRect.Left), uintptr(c.savedRect.Bottom-c.savedRect.Top),
				swpNoZOrder|swpNoActivate,
			)
		} else {
			procSetWindowPos.Call(
				c.hwnd, 0,
				uintptr(c.savedRect.Left), uintptr(c.savedRect.Top),
				uintptr(c.savedRect.Right-c.savedRect.Left), uintptr(c.savedRect.Bottom-c.savedRect.Top),
				swpNoZOrder,
			)
			procSetForegroundWindow.Call(c.hwnd)
			c.SetAlwaysOnTop(c.alwaysOnTop)
		}

		// Cleanly restore opacity and remove layered style when idle.
		procSetLayeredWindowAttributes.Call(c.hwnd, 0, 255, lwaAlpha)
		exStyle := getWindowLongPtr(c.hwnd, gwlExStyle)
		if (exStyle & wsExLayered) != 0 {
			setWindowLongPtr(c.hwnd, gwlExStyle, exStyle&^wsExLayered)
		}
		if c.callbacks.OnResizeNeeded != nil {
			c.callbacks.OnResizeNeeded()
		}
	}
}

func (c *Controller) stepResizeAnimation() {
	if !c.resizeAnim.active {
		procKillTimer.Call(c.hwnd, timerResizeID)
		return
	}

	c.resizeAnim.step++
	t := float64(c.resizeAnim.step) / float64(resizeSteps)
	if t > 1 {
		t = 1
	}
	inv := 1.0 - t
	ease := 1.0 - inv*inv*inv

	h := c.resizeAnim.startH + int32(float64(c.resizeAnim.targetH-c.resizeAnim.startH)*ease)
	procSetWindowPos.Call(c.hwnd, 0, 0, 0, uintptr(c.resizeAnim.winW), uintptr(h), swpNoMove|swpNoZOrder|swpNoActivate)

	if c.resizeAnim.step >= resizeSteps {
		procKillTimer.Call(c.hwnd, timerResizeID)
		c.resizeAnim.active = false
		procSetWindowPos.Call(c.hwnd, 0, 0, 0, uintptr(c.resizeAnim.winW), uintptr(c.resizeAnim.targetH), swpNoMove|swpNoZOrder|swpNoActivate)
		var rc rect
		procGetWindowRect.Call(c.hwnd, uintptr(unsafe.Pointer(&rc)))
		c.savedRect = rc
		if c.callbacks.OnResizeNeeded != nil {
			c.callbacks.OnResizeNeeded()
		}
	}
}

func lerpRect(a, b rect, t float64) rect {
	lerp := func(x, y int32) int32 {
		return x + int32(float64(y-x)*t)
	}
	return rect{
		Left:   lerp(a.Left, b.Left),
		Top:    lerp(a.Top, b.Top),
		Right:  lerp(a.Right, b.Right),
		Bottom: lerp(a.Bottom, b.Bottom),
	}
}
