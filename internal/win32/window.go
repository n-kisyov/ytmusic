//go:build windows

package win32

import (
	"sync"
	"unsafe"

	"golang.org/x/sys/windows"
)

const (
	wmDestroy    = 0x0002
	wmClose      = 0x0010
	wmCommand    = 0x0111
	wmSysCommand = 0x0112
	wmTimer      = 0x0113
	wmLButtonUp  = 0x0202
	wmRButtonUp  = 0x0205
	wmContextMenu = 0x007B
	wmTrayIcon   = 0x800A // WM_APP + 10 (avoids go-webview2's WM_APP = 0x8000)
	wmRestoreApp = 0x800B // WM_APP + 11 (posted by second instance to restore existing window)

	scMinimize = 0xF020

	gwlStyle   = ^uintptr(15) // -16
	gwlExStyle = ^uintptr(19) // -20
	gwlpWndProc = ^uintptr(3) // -4

	wsThickFrame  = 0x00040000
	wsMaximizeBox = 0x00010000

	dwmwaUseImmersiveDarkMode    = 20
	dwmwaWindowCornerPreference  = 33
	dwmwcpRound                  = 2

	hwndTopMost   = ^uintptr(0) // -1
	hwndNoTopMost = ^uintptr(1) // -2
)

type rect struct {
	Left   int32
	Top    int32
	Right  int32
	Bottom int32
}

type point struct {
	X int32
	Y int32
}

// Callbacks connects Win32 window/tray events back into the application layer.
type Callbacks struct {
	OnPlayPause         func()
	OnNext              func()
	OnPrev              func()
	OnToggleVideo       func()
	IsVideoEnabled      func() bool
	OnToggleAlwaysOnTop func()
	IsAlwaysOnTop       func() bool
	OnExit              func()
	OnResizeNeeded      func()
}

// Controller manages the native Win32 window styles, DWM attributes, tray icon, and animations.
type Controller struct {
	hwnd        uintptr
	origWndProc uintptr
	hIcon       uintptr
	trayAdded   bool
	hidden      bool
	alwaysOnTop bool
	quitting    bool
	savedRect   rect
	resizeAnim  resizeAnimState
	trayAnim    trayAnimState
	callbacks   Callbacks
}

var (
	user32                        = windows.NewLazySystemDLL("user32.dll")
	procGetWindowLongPtrW         = user32.NewProc("GetWindowLongPtrW")
	procSetWindowLongPtrW         = user32.NewProc("SetWindowLongPtrW")
	procCallWindowProcW           = user32.NewProc("CallWindowProcW")
	procSetWindowPos              = user32.NewProc("SetWindowPos")
	procGetWindowRect             = user32.NewProc("GetWindowRect")
	procShowWindow                = user32.NewProc("ShowWindow")
	procSendMessageW              = user32.NewProc("SendMessageW")
	procPostMessageW              = user32.NewProc("PostMessageW")
	procSetTimer                  = user32.NewProc("SetTimer")
	procKillTimer                 = user32.NewProc("KillTimer")
	procSetLayeredWindowAttributes = user32.NewProc("SetLayeredWindowAttributes")
	procAdjustWindowRectEx        = user32.NewProc("AdjustWindowRectEx")
	procAdjustWindowRectExForDpi  = user32.NewProc("AdjustWindowRectExForDpi")
	procGetDpiForWindow           = user32.NewProc("GetDpiForWindow")
	procFindWindowW               = user32.NewProc("FindWindowW")
	procFindWindowExW             = user32.NewProc("FindWindowExW")
	procSystemParametersInfoW     = user32.NewProc("SystemParametersInfoW")
	procGetSystemMetrics          = user32.NewProc("GetSystemMetrics")
	procDestroyWindow             = user32.NewProc("DestroyWindow")

	dwmapi                  = windows.NewLazySystemDLL("dwmapi.dll")
	procDwmSetWindowAttribute = dwmapi.NewProc("DwmSetWindowAttribute")
)

var (
	ctrlMu      sync.RWMutex
	controllers = make(map[uintptr]*Controller)
	wndProcCB   = windows.NewCallback(subclassWndProc)
)

// NewController attaches Win32 non-resizable styling, Windows 11 DWM theme, tray icon, and WndProc hooks.
func NewController(hwnd unsafe.Pointer, cb Callbacks) *Controller {
	h := uintptr(hwnd)
	c := &Controller{
		hwnd:      h,
		callbacks: cb,
	}

	ctrlMu.Lock()
	controllers[h] = c
	ctrlMu.Unlock()

	c.applyWindowChrome()
	c.initTray()

	orig := setWindowLongPtr(h, gwlpWndProc, wndProcCB)
	c.origWndProc = orig

	var rc rect
	procGetWindowRect.Call(h, uintptr(unsafe.Pointer(&rc)))
	c.savedRect = rc

	return c
}

func (c *Controller) applyWindowChrome() {
	// Make window strictly non-resizable and disable maximize button.
	style := getWindowLongPtr(c.hwnd, gwlStyle)
	style &^= (wsThickFrame | wsMaximizeBox)
	setWindowLongPtr(c.hwnd, gwlStyle, style)
	procSetWindowPos.Call(c.hwnd, 0, 0, 0, 0, 0, swpNoMove|swpNoSize|swpNoZOrder|swpNoActivate|swpFrameChanged)

	// Enable Windows 11 dark title bar & rounded corners.
	if procDwmSetWindowAttribute.Find() == nil {
		dark := uint32(1)
		procDwmSetWindowAttribute.Call(c.hwnd, dwmwaUseImmersiveDarkMode, uintptr(unsafe.Pointer(&dark)), unsafe.Sizeof(dark))
		round := uint32(dwmwcpRound)
		procDwmSetWindowAttribute.Call(c.hwnd, dwmwaWindowCornerPreference, uintptr(unsafe.Pointer(&round)), unsafe.Sizeof(round))
	}
}

// SetWindowPosition moves the window to (x, y) if valid and on-screen, or centers on the primary work area.
func (c *Controller) SetWindowPosition(x, y int) {
	var rc rect
	procGetWindowRect.Call(c.hwnd, uintptr(unsafe.Pointer(&rc)))
	w := int(rc.Right - rc.Left)
	h := int(rc.Bottom - rc.Top)

	var wa rect
	procSystemParametersInfoW.Call(48 /* SPI_GETWORKAREA */, 0, uintptr(unsafe.Pointer(&wa)), 0)

	// Validate saved coordinates against virtual desktop bounds.
	vx, _, _ := procGetSystemMetrics.Call(76 /* SM_XVIRTUALSCREEN */)
	vy, _, _ := procGetSystemMetrics.Call(77 /* SM_YVIRTUALSCREEN */)
	vw, _, _ := procGetSystemMetrics.Call(78 /* SM_CXVIRTUALSCREEN */)
	vh, _, _ := procGetSystemMetrics.Call(79 /* SM_CYVIRTUALSCREEN */)

	minX := int(int32(vx))
	minY := int(int32(vy))
	maxX := minX + int(int32(vw)) - 100
	maxY := minY + int(int32(vh)) - 60

	if (x == -1 && y == -1) || x < minX || y < minY || x > maxX || y > maxY {
		x = int(wa.Left) + (int(wa.Right-wa.Left)-w)/2
		y = int(wa.Top) + (int(wa.Bottom-wa.Top)-h)/2
	}

	procSetWindowPos.Call(c.hwnd, 0, uintptr(x), uintptr(y), 0, 0, swpNoSize|swpNoZOrder|swpNoActivate)
	procGetWindowRect.Call(c.hwnd, uintptr(unsafe.Pointer(&rc)))
	c.savedRect = rc
}

// GetWindowPosition returns the last known top-left screen coordinates of the window.
func (c *Controller) GetWindowPosition() (int, int) {
	if !c.hidden && c.trayAnim.mode == trayAnimNone {
		var rc rect
		procGetWindowRect.Call(c.hwnd, uintptr(unsafe.Pointer(&rc)))
		if (rc.Right - rc.Left) > 80 {
			c.savedRect = rc
		}
	}
	return int(c.savedRect.Left), int(c.savedRect.Top)
}

// SetClientSize resizes the fixed-width window to the given logical client size (optionally animated).
func (c *Controller) SetClientSize(logicalW, logicalH int, animate bool) {
	dpi := c.getDPI()
	physW := (logicalW*dpi + 48) / 96
	physH := (logicalH*dpi + 48) / 96

	style := getWindowLongPtr(c.hwnd, gwlStyle)
	exStyle := getWindowLongPtr(c.hwnd, gwlExStyle)
	r := rect{Left: 0, Top: 0, Right: int32(physW), Bottom: int32(physH)}

	if procAdjustWindowRectExForDpi.Find() == nil {
		procAdjustWindowRectExForDpi.Call(uintptr(unsafe.Pointer(&r)), style, 0, exStyle, uintptr(dpi))
	} else {
		procAdjustWindowRectEx.Call(uintptr(unsafe.Pointer(&r)), style, 0, exStyle)
	}

	winW := r.Right - r.Left
	winH := r.Bottom - r.Top

	if c.hidden {
		c.savedRect.Right = c.savedRect.Left + winW
		c.savedRect.Bottom = c.savedRect.Top + winH
		procSetWindowPos.Call(c.hwnd, 0, 0, 0, uintptr(winW), uintptr(winH), swpNoMove|swpNoZOrder|swpNoActivate)
		return
	}

	var cur rect
	procGetWindowRect.Call(c.hwnd, uintptr(unsafe.Pointer(&cur)))
	curH := cur.Bottom - cur.Top

	if !animate || curH == winH || (cur.Right-cur.Left) == 0 {
		c.resizeAnim.active = false
		procKillTimer.Call(c.hwnd, timerResizeID)
		procSetWindowPos.Call(c.hwnd, 0, 0, 0, uintptr(winW), uintptr(winH), swpNoMove|swpNoZOrder|swpNoActivate)
		procGetWindowRect.Call(c.hwnd, uintptr(unsafe.Pointer(&c.savedRect)))
		if c.callbacks.OnResizeNeeded != nil {
			c.callbacks.OnResizeNeeded()
		}
		return
	}

	c.resizeAnim = resizeAnimState{
		active:  true,
		step:    0,
		startH:  curH,
		targetH: winH,
		winW:    winW,
	}
	procSetTimer.Call(c.hwnd, timerResizeID, animTickMs, 0)
}

// SetAlwaysOnTop toggles whether the window stays above non-topmost windows.
func (c *Controller) SetAlwaysOnTop(enabled bool) {
	c.alwaysOnTop = enabled
	insertAfter := hwndNoTopMost
	if enabled {
		insertAfter = hwndTopMost
	}
	procSetWindowPos.Call(c.hwnd, insertAfter, 0, 0, 0, 0, swpNoMove|swpNoSize|swpNoActivate)
}

// BeginQuit marks the window as exiting, cleans up tray icon, and triggers WM_CLOSE destruction.
func (c *Controller) BeginQuit() {
	c.quitting = true
	procKillTimer.Call(c.hwnd, timerResizeID)
	procKillTimer.Call(c.hwnd, timerTrayAnimID)
	c.removeTray()
	procDestroyWindow.Call(c.hwnd)
}

func (c *Controller) getDPI() int {
	if procGetDpiForWindow.Find() == nil {
		dpi, _, _ := procGetDpiForWindow.Call(c.hwnd)
		if dpi > 0 {
			return int(dpi)
		}
	}
	return 96
}

func subclassWndProc(hwnd, msg, wParam, lParam uintptr) uintptr {
	ctrlMu.RLock()
	c := controllers[hwnd]
	ctrlMu.RUnlock()
	if c == nil {
		return 0
	}

	switch msg {
	case wmClose:
		if !c.quitting {
			c.MinimizeToTray()
			return 0
		}
		c.removeTray()

	case wmSysCommand:
		if (wParam & 0xFFF0) == scMinimize {
			c.MinimizeToTray()
			return 0
		}

	case wmTrayIcon:
		evt := lParam & 0xFFFF
		switch evt {
		case wmLButtonUp:
			c.ToggleTrayVisibility()
			return 0
		case wmRButtonUp, wmContextMenu:
			c.showTrayMenu()
			return 0
		}

	case wmRestoreApp:
		c.RestoreFromTray()
		return 0

	case wmCommand:
		cmdID := wParam & 0xFFFF
		switch cmdID {
		case cmdOpen:
			c.ToggleTrayVisibility()
			return 0
		case cmdPlayPause:
			if c.callbacks.OnPlayPause != nil {
				c.callbacks.OnPlayPause()
			}
			return 0
		case cmdNext:
			if c.callbacks.OnNext != nil {
				c.callbacks.OnNext()
			}
			return 0
		case cmdPrev:
			if c.callbacks.OnPrev != nil {
				c.callbacks.OnPrev()
			}
			return 0
		case cmdToggleVideo:
			if c.callbacks.OnToggleVideo != nil {
				c.callbacks.OnToggleVideo()
			}
			return 0
		case cmdToggleAlwaysOnTop:
			if c.callbacks.OnToggleAlwaysOnTop != nil {
				c.callbacks.OnToggleAlwaysOnTop()
			}
			return 0
		case cmdExit:
			if c.callbacks.OnExit != nil {
				c.callbacks.OnExit()
			} else {
				c.BeginQuit()
			}
			return 0
		}

	case wmTimer:
		switch wParam {
		case timerResizeID:
			c.stepResizeAnimation()
			return 0
		case timerTrayAnimID:
			c.stepTrayAnimation()
			return 0
		}

	case wmDestroy:
		c.removeTray()
		ctrlMu.Lock()
		delete(controllers, hwnd)
		ctrlMu.Unlock()
	}

	ret, _, _ := procCallWindowProcW.Call(c.origWndProc, hwnd, msg, wParam, lParam)
	return ret
}

func getWindowLongPtr(hwnd uintptr, idx uintptr) uintptr {
	r, _, _ := procGetWindowLongPtrW.Call(hwnd, idx)
	return r
}

func setWindowLongPtr(hwnd uintptr, idx uintptr, val uintptr) uintptr {
	r, _, _ := procSetWindowLongPtrW.Call(hwnd, idx, val)
	return r
}
