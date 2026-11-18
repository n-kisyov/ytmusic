//go:build windows

package win32

import (
	"math"
	"time"
	"unsafe"
)

const (
	timerResizeID   = 1
	timerTrayAnimID = 2

	animTickMs         = 7 // ~142 FPS with timeBeginPeriod(1)
	trayAnimDuration   = 185 * time.Millisecond
	resizeAnimDuration = 140 * time.Millisecond

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
	active    bool
	startTime time.Time
	startH    int32
	targetH   int32
	winW      int32
}

type trayAnimMode int

const (
	trayAnimNone trayAnimMode = iota
	trayAnimMinimize
	trayAnimRestore
)

type trayAnimState struct {
	mode      trayAnimMode
	startTime time.Time
	fromX     int32
	fromY     int32
	toX       int32
	toY       int32
}

func (c *Controller) beginHighResTimer() {
	if !c.timerPeriodOn && procTimeBeginPeriod.Find() == nil {
		procTimeBeginPeriod.Call(1)
		c.timerPeriodOn = true
	}
}

func (c *Controller) endHighResTimer() {
	if c.timerPeriodOn && procTimeEndPeriod.Find() == nil {
		procTimeEndPeriod.Call(1)
		c.timerPeriodOn = false
	}
}

func (c *Controller) trayGlideOffset(base rect) (int32, int32) {
	tray := c.getTrayIconRect()
	scx := float64(base.Left+base.Right) / 2.0
	scy := float64(base.Top+base.Bottom) / 2.0
	tcx := float64(tray.Left+tray.Right) / 2.0
	tcy := float64(tray.Top+tray.Bottom) / 2.0

	dx := tcx - scx
	dy := tcy - scy
	dist := math.Hypot(dx, dy)
	if dist < 1 {
		return 0, 80
	}
	glide := math.Min(110.0, dist)
	return int32(math.Round(dx / dist * glide)), int32(math.Round(dy / dist * glide))
}

// MinimizeToTray smoothly glides and fades the window toward the system tray icon at high FPS.
func (c *Controller) MinimizeToTray() {
	c.revealed = true
	if c.hidden || c.trayAnim.mode == trayAnimMinimize {
		return
	}

	if c.resizeAnim.active {
		c.resizeAnim.active = false
		procKillTimer.Call(c.hwnd, timerResizeID)
	}

	var cur rect
	procGetWindowRect.Call(c.hwnd, uintptr(unsafe.Pointer(&cur)))
	if (cur.Right - cur.Left) > 80 && (cur.Bottom - cur.Top) > 80 {
		c.savedRect = cur
	}

	offX, offY := c.trayGlideOffset(c.savedRect)

	exStyle := getWindowLongPtr(c.hwnd, gwlExStyle)
	if (exStyle & wsExLayered) == 0 {
		setWindowLongPtr(c.hwnd, gwlExStyle, exStyle|wsExLayered)
	}
	procSetLayeredWindowAttributes.Call(c.hwnd, 0, 255, lwaAlpha)

	c.beginHighResTimer()
	c.trayAnim = trayAnimState{
		mode:      trayAnimMinimize,
		startTime: time.Now(),
		fromX:     c.savedRect.Left,
		fromY:     c.savedRect.Top,
		toX:       c.savedRect.Left + offX,
		toY:       c.savedRect.Top + offY,
	}
	procSetTimer.Call(c.hwnd, timerTrayAnimID, animTickMs, 0)
}

// RestoreFromTray smoothly glides and fades the window back from the system tray at high FPS.
func (c *Controller) RestoreFromTray() {
	c.revealed = true
	if !c.hidden && c.trayAnim.mode != trayAnimMinimize {
		procSetForegroundWindow.Call(c.hwnd)
		return
	}

	if (c.savedRect.Right - c.savedRect.Left) <= 80 {
		var cur rect
		procGetWindowRect.Call(c.hwnd, uintptr(unsafe.Pointer(&cur)))
		c.savedRect = cur
	}

	offX, offY := c.trayGlideOffset(c.savedRect)
	startX := c.savedRect.Left + offX
	startY := c.savedRect.Top + offY

	exStyle := getWindowLongPtr(c.hwnd, gwlExStyle)
	if (exStyle & wsExLayered) == 0 {
		setWindowLongPtr(c.hwnd, gwlExStyle, exStyle|wsExLayered)
	}
	procSetLayeredWindowAttributes.Call(c.hwnd, 0, 0, lwaAlpha)
	procSetWindowPos.Call(
		c.hwnd, 0,
		uintptr(startX), uintptr(startY),
		0, 0,
		swpNoSize|swpNoZOrder|swpNoActivate,
	)
	procShowWindow.Call(c.hwnd, swShow)

	c.hidden = false
	c.beginHighResTimer()
	c.trayAnim = trayAnimState{
		mode:      trayAnimRestore,
		startTime: time.Now(),
		fromX:     startX,
		fromY:     startY,
		toX:       c.savedRect.Left,
		toY:       c.savedRect.Top,
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
		if !c.resizeAnim.active {
			c.endHighResTimer()
		}
		return
	}

	t := float64(time.Since(c.trayAnim.startTime)) / float64(trayAnimDuration)
	if t < 0 {
		t = 0
	}
	if t > 1 {
		t = 1
	}

	var ease float64
	var alpha uint8
	if c.trayAnim.mode == trayAnimMinimize {
		// Smooth cubic ease-in glide + fade out.
		ease = t * t * t
		alpha = uint8(math.Round(255.0 * (1.0 - t*t)))
	} else {
		// Smooth cubic ease-out glide + fade in.
		inv := 1.0 - t
		ease = 1.0 - inv*inv*inv
		alpha = uint8(math.Round(255.0 * ease))
	}

	x := c.trayAnim.fromX + int32(math.Round(float64(c.trayAnim.toX-c.trayAnim.fromX)*ease))
	y := c.trayAnim.fromY + int32(math.Round(float64(c.trayAnim.toY-c.trayAnim.fromY)*ease))

	procSetLayeredWindowAttributes.Call(c.hwnd, 0, uintptr(alpha), lwaAlpha)
	procSetWindowPos.Call(c.hwnd, 0, uintptr(x), uintptr(y), 0, 0, swpNoSize|swpNoZOrder|swpNoActivate)

	if t >= 1 {
		procKillTimer.Call(c.hwnd, timerTrayAnimID)
		completedMode := c.trayAnim.mode
		c.trayAnim.mode = trayAnimNone
		if !c.resizeAnim.active {
			c.endHighResTimer()
		}

		if completedMode == trayAnimMinimize {
			procShowWindow.Call(c.hwnd, swHide)
			c.hidden = true
			// Snap back to saved position while hidden so next restore starts cleanly.
			procSetWindowPos.Call(
				c.hwnd, 0,
				uintptr(c.savedRect.Left), uintptr(c.savedRect.Top),
				0, 0,
				swpNoSize|swpNoZOrder|swpNoActivate,
			)
		} else {
			procSetWindowPos.Call(
				c.hwnd, 0,
				uintptr(c.savedRect.Left), uintptr(c.savedRect.Top),
				0, 0,
				swpNoSize|swpNoZOrder,
			)
			procSetForegroundWindow.Call(c.hwnd)
			c.SetAlwaysOnTop(c.alwaysOnTop)
		}

		procSetLayeredWindowAttributes.Call(c.hwnd, 0, 255, lwaAlpha)
		exStyle := getWindowLongPtr(c.hwnd, gwlExStyle)
		if (exStyle & wsExLayered) != 0 {
			setWindowLongPtr(c.hwnd, gwlExStyle, exStyle&^wsExLayered)
		}
	}
}

func (c *Controller) stepResizeAnimation() {
	if !c.resizeAnim.active {
		procKillTimer.Call(c.hwnd, timerResizeID)
		if c.trayAnim.mode == trayAnimNone {
			c.endHighResTimer()
		}
		return
	}

	t := float64(time.Since(c.resizeAnim.startTime)) / float64(resizeAnimDuration)
	if t < 0 {
		t = 0
	}
	if t > 1 {
		t = 1
	}
	inv := 1.0 - t
	ease := 1.0 - inv*inv*inv

	h := c.resizeAnim.startH + int32(math.Round(float64(c.resizeAnim.targetH-c.resizeAnim.startH)*ease))
	procSetWindowPos.Call(c.hwnd, 0, 0, 0, uintptr(c.resizeAnim.winW), uintptr(h), swpNoMove|swpNoZOrder|swpNoActivate)

	if t >= 1 {
		procKillTimer.Call(c.hwnd, timerResizeID)
		c.resizeAnim.active = false
		if c.trayAnim.mode == trayAnimNone {
			c.endHighResTimer()
		}
		procSetWindowPos.Call(c.hwnd, 0, 0, 0, uintptr(c.resizeAnim.winW), uintptr(c.resizeAnim.targetH), swpNoMove|swpNoZOrder|swpNoActivate)
		var rc rect
		procGetWindowRect.Call(c.hwnd, uintptr(unsafe.Pointer(&rc)))
		c.savedRect = rc
		if c.callbacks.OnResizeNeeded != nil {
			c.callbacks.OnResizeNeeded()
		}
	}
}
