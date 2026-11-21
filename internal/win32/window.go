//go:build windows

package win32

import (
	"fmt"
	"strings"
	"sync"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

const (
	wmDestroy         = 0x0002
	wmClose           = 0x0010
	wmQueryEndSession = 0x0011
	wmEraseBkgnd      = 0x0014
	wmEndSession      = 0x0016
	wmShowWindow      = 0x0018
	wmWindowPosChanging = 0x0046
	wmCommand         = 0x0111
	wmSysCommand      = 0x0112
	wmTimer           = 0x0113
	wmHotKey          = 0x0312
	wmPowerBroadcast  = 0x0218
	wmLButtonUp       = 0x0202
	wmRButtonUp       = 0x0205
	wmMButtonUp       = 0x0208
	wmContextMenu     = 0x007B
	wmDpiChanged      = 0x02E0
	wmNcCalcSize      = 0x0083
	wmNcLButtonDown   = 0x00A1
	htCaption         = 2
	wmTrayIcon        = 0x800A // WM_APP + 10 (avoids go-webview2's WM_APP = 0x8000)
	wmRestoreApp      = 0x800B // WM_APP + 11 (posted by second instance to restore existing window)
	wmResetStats      = 0x800C // WM_APP + 12 (posted by --reset-stats CLI flag)

	pbtApmSuspend    = 0x0004
	pbtApmResumeAuto = 0x0012

	scMinimize = 0xF020

	vkMediaNextTrack = 0xB0
	vkMediaPrevTrack = 0xB1
	vkMediaPlayPause = 0xB3

	hkPlayPause = 1
	hkNext      = 2
	hkPrev      = 3

	gwlStyle   = ^uintptr(15) // -16
	gwlExStyle = ^uintptr(19) // -20
	gwlpWndProc = ^uintptr(3) // -4

	wsCaption     = 0x00C00000
	wsThickFrame  = 0x00040000
	wsMaximizeBox = 0x00010000

	dwmwaCloak                  = 13
	dwmwaUseImmersiveDarkMode   = 20
	dwmwaWindowCornerPreference = 33
	dwmwcpRound                 = 2

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
	OnPlayPause          func()
	OnNext               func()
	OnPrev               func()
	OnToggleVideo        func()
	IsVideoEnabled       func() bool
	OnToggleAlwaysOnTop  func()
	IsAlwaysOnTop        func() bool
	OnToggleTheme        func()
	IsLightTheme         func() bool
	OnToggleShuffle      func()
	IsShuffleEnabled     func() bool
	OnCycleRepeat        func()
	GetRepeatMode        func() string
	OnSetSleepTimer      func(minutes int)
	GetSleepTimerMinutes func() int
	OnExit               func()
	OnResizeNeeded       func()
	OnSystemSuspend      func()
	OnSystemResume       func()
	OnResetStats         func()
}

// Controller manages the native Win32 window styles, DWM attributes, tray icon, and animations.
type Controller struct {
	hwnd          uintptr
	origWndProc   uintptr
	hIcon         uintptr
	trayAdded     bool
	hidden        bool
	alwaysOnTop   bool
	isDark        bool
	revealed      bool
	timerPeriodOn bool
	quitting      bool
	savedRect     rect
	resizeAnim    resizeAnimState
	trayAnim      trayAnimState
	callbacks     Callbacks
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
	procGetClientRect             = user32.NewProc("GetClientRect")
	procFillRect                  = user32.NewProc("FillRect")
	procSetTimer                  = user32.NewProc("SetTimer")
	procKillTimer                 = user32.NewProc("KillTimer")
	procSetLayeredWindowAttributes = user32.NewProc("SetLayeredWindowAttributes")
	procAdjustWindowRectEx        = user32.NewProc("AdjustWindowRectEx")
	procAdjustWindowRectExForDpi  = user32.NewProc("AdjustWindowRectExForDpi")
	procGetDpiForWindow           = user32.NewProc("GetDpiForWindow")
	procGetDpiForSystem           = user32.NewProc("GetDpiForSystem")
	procFindWindowW               = user32.NewProc("FindWindowW")
	procFindWindowExW             = user32.NewProc("FindWindowExW")
	procSystemParametersInfoW     = user32.NewProc("SystemParametersInfoW")
	procShellExecuteW             = shell32.NewProc("ShellExecuteW")
	procGetSystemMetrics          = user32.NewProc("GetSystemMetrics")
	procCopyRect                  = user32.NewProc("CopyRect")
	procDestroyWindow             = user32.NewProc("DestroyWindow")
	procRegisterHotKey            = user32.NewProc("RegisterHotKey")
	procUnregisterHotKey          = user32.NewProc("UnregisterHotKey")
	procSetWindowsHookExW         = user32.NewProc("SetWindowsHookExW")
	procUnhookWindowsHookEx       = user32.NewProc("UnhookWindowsHookEx")
	procCallNextHookEx            = user32.NewProc("CallNextHookEx")
	procGetClassNameW             = user32.NewProc("GetClassNameW")
	procMonitorFromPoint          = user32.NewProc("MonitorFromPoint")
	procMonitorFromRect           = user32.NewProc("MonitorFromRect")
	procGetMonitorInfoW           = user32.NewProc("GetMonitorInfoW")
	procIsIconic                  = user32.NewProc("IsIconic")
	procDefWindowProcW            = user32.NewProc("DefWindowProcW")
	procReleaseCapture            = user32.NewProc("ReleaseCapture")

	shcore               = windows.NewLazySystemDLL("shcore.dll")
	procGetDpiForMonitor = shcore.NewProc("GetDpiForMonitor")

	kernel32               = windows.NewLazySystemDLL("kernel32.dll")
	procGetCurrentThreadId = kernel32.NewProc("GetCurrentThreadId")

	winmm               = windows.NewLazySystemDLL("winmm.dll")
	procTimeBeginPeriod = winmm.NewProc("timeBeginPeriod")
	procTimeEndPeriod   = winmm.NewProc("timeEndPeriod")

	dwmapi                           = windows.NewLazySystemDLL("dwmapi.dll")
	procDwmSetWindowAttribute        = dwmapi.NewProc("DwmSetWindowAttribute")
	procDwmExtendFrameIntoClientArea = dwmapi.NewProc("DwmExtendFrameIntoClientArea")

	procCreateSolidBrush = gdi32.NewProc("CreateSolidBrush")

	bgBrushMu    sync.Mutex
	darkBgBrush  uintptr
	lightBgBrush uintptr
)

func bgBrush(dark bool) uintptr {
	bgBrushMu.Lock()
	defer bgBrushMu.Unlock()
	if dark {
		if darkBgBrush == 0 {
			// Win32 COLORREF is 0x00BBGGRR (#0f0f0f).
			darkBgBrush, _, _ = procCreateSolidBrush.Call(0x000F0F0F)
		}
		return darkBgBrush
	}
	if lightBgBrush == 0 {
		// Win32 COLORREF is 0x00BBGGRR (#ffffff).
		lightBgBrush, _, _ = procCreateSolidBrush.Call(0x00FFFFFF)
	}
	return lightBgBrush
}

func setWindowCloak(hwnd uintptr, cloaked bool) {
	if hwnd == 0 || procDwmSetWindowAttribute.Find() != nil {
		return
	}
	val := uint32(0)
	if cloaked {
		val = 1
	}
	procDwmSetWindowAttribute.Call(hwnd, dwmwaCloak, uintptr(unsafe.Pointer(&val)), unsafe.Sizeof(val))
}

func fillWindowBg(hwnd, hdc uintptr, dark bool) {
	var rc rect
	procGetClientRect.Call(hwnd, uintptr(unsafe.Pointer(&rc)))
	procFillRect.Call(hdc, uintptr(unsafe.Pointer(&rc)), bgBrush(dark))
}

type margins struct {
	CxLeftWidth    int32
	CxRightWidth   int32
	CyTopHeight    int32
	CyBottomHeight int32
}

type createStructW struct {
	LpCreateParams uintptr
	HInstance      uintptr
	HMenu          uintptr
	HwndParent     uintptr
	Cy             int32
	Cx             int32
	Y              int32
	X              int32
	Style          int32
	LpszName       *uint16
	LpszClass      uintptr
	DwExStyle      uint32
}

type cbtCreateWndW struct {
	Lpcs            *createStructW
	HwndInsertAfter uintptr
}

var (
	ctrlMu           sync.RWMutex
	controllers      = make(map[uintptr]*Controller)
	wndProcCB        = windows.NewCallback(subclassWndProc)
	cbtHookCB        uintptr
	startupWndProcCB uintptr
	startupCBOnce    sync.Once
	startupHookMu    sync.Mutex
	startupHookH     uintptr
	startupDark      bool
	startupRect      rect
	startupTarget    uintptr
	startupOrigProc  uintptr
)

type monitorInfo struct {
	CbSize    uint32
	RcMonitor rect
	RcWork    rect
	DwFlags   uint32
}

func monitorDPIAndWorkArea(savedX, savedY int) (int, rect) {
	dpi := 96
	if procGetDpiForSystem.Find() == nil {
		if d, _, _ := procGetDpiForSystem.Call(); d > 0 {
			dpi = int(d)
		}
	}

	var wa rect
	procSystemParametersInfoW.Call(48 /* SPI_GETWORKAREA */, 0, uintptr(unsafe.Pointer(&wa)), 0)

	if procMonitorFromPoint.Find() == nil {
		var hMon uintptr
		if savedX != -1 || savedY != -1 {
			ptPacked := uintptr(uint32(int32(savedX))) | (uintptr(uint32(int32(savedY))) << 32)
			hMon, _, _ = procMonitorFromPoint.Call(ptPacked, 0 /* MONITOR_DEFAULTTONULL */)
		}
		if hMon == 0 {
			hMon, _, _ = procMonitorFromPoint.Call(0, 1 /* MONITOR_DEFAULTTOPRIMARY */)
		}
		if hMon != 0 {
			if procGetDpiForMonitor.Find() == nil {
				var dpiX, dpiY uint32
				hr, _, _ := procGetDpiForMonitor.Call(hMon, 0 /* MDT_EFFECTIVE_DPI */, uintptr(unsafe.Pointer(&dpiX)), uintptr(unsafe.Pointer(&dpiY)))
				if int32(hr) >= 0 && dpiX > 0 {
					dpi = int(dpiX)
				}
			}
			if procGetMonitorInfoW.Find() == nil {
				mi := monitorInfo{CbSize: uint32(unsafe.Sizeof(monitorInfo{}))}
				if ok, _, _ := procGetMonitorInfoW.Call(hMon, uintptr(unsafe.Pointer(&mi))); ok != 0 {
					wa = mi.RcWork
				}
			}
		}
	}

	return dpi, wa
}

func isRectOnAnyMonitor(r rect) bool {
	if procMonitorFromRect.Find() == nil {
		hMon, _, _ := procMonitorFromRect.Call(uintptr(unsafe.Pointer(&r)), 0 /* MONITOR_DEFAULTTONULL */)
		return hMon != 0
	}
	if procMonitorFromPoint.Find() == nil {
		ptPacked := uintptr(uint32(r.Left)) | (uintptr(uint32(r.Top)) << 32)
		hMon, _, _ := procMonitorFromPoint.Call(ptPacked, 0 /* MONITOR_DEFAULTTONULL */)
		return hMon != 0
	}
	return true
}

// ComputeInitialWindowRect computes the exact physical window bounds for the given logical
// client size and saved top-left position (centering on primary work area if unset or offscreen).
func ComputeInitialWindowRect(savedX, savedY, logicalW, logicalH int) (int32, int32, int32, int32) {
	dpi, wa := monitorDPIAndWorkArea(savedX, savedY)
	physW := (logicalW*dpi + 48) / 96
	physH := (logicalH*dpi + 48) / 96

	style := uintptr(0x00CF0000) &^ (wsCaption | wsThickFrame | wsMaximizeBox)
	exStyle := uintptr(wsExLayered)
	r := rect{Left: 0, Top: 0, Right: int32(physW), Bottom: int32(physH)}
	if procAdjustWindowRectExForDpi.Find() == nil {
		procAdjustWindowRectExForDpi.Call(uintptr(unsafe.Pointer(&r)), style, 0, exStyle, uintptr(dpi))
	} else {
		procAdjustWindowRectEx.Call(uintptr(unsafe.Pointer(&r)), style, 0, exStyle)
	}
	winW := int(r.Right - r.Left)
	winH := int(r.Bottom - r.Top)

	vx, _, _ := procGetSystemMetrics.Call(76 /* SM_XVIRTUALSCREEN */)
	vy, _, _ := procGetSystemMetrics.Call(77 /* SM_YVIRTUALSCREEN */)
	vw, _, _ := procGetSystemMetrics.Call(78 /* SM_CXVIRTUALSCREEN */)
	vh, _, _ := procGetSystemMetrics.Call(79 /* SM_CYVIRTUALSCREEN */)

	minX := int(int32(vx))
	minY := int(int32(vy))
	maxX := minX + int(int32(vw)) - 100
	maxY := minY + int(int32(vh)) - 60

	x, y := savedX, savedY
	titleStrip := rect{
		Left:   int32(x),
		Top:    int32(y),
		Right:  int32(x + min(winW, 120)),
		Bottom: int32(y + 40),
	}
	if (x == -1 && y == -1) || x < minX || y < minY || x > maxX || y > maxY || !isRectOnAnyMonitor(titleStrip) {
		x = int(wa.Left) + (int(wa.Right-wa.Left)-winW)/2
		y = int(wa.Top) + (int(wa.Bottom-wa.Top)-winH)/2
	}
	return int32(x), int32(y), int32(winW), int32(winH)
}

// InstallStartupHook installs a thread-local WH_CBT hook that applies WS_EX_LAYERED,
// exact saved position/size, non-resizable styles, and DWM theme directly to CREATESTRUCTW
// before CreateWindowExW / ShowWindow runs, preventing any startup white flash or jump.
func InstallStartupHook(dark bool, savedX, savedY, logicalW, logicalH int) {
	startupCBOnce.Do(func() {
		cbtHookCB = windows.NewCallback(startupCBTHookProc)
		startupWndProcCB = windows.NewCallback(startupPreShowWndProc)
	})

	startupHookMu.Lock()
	defer startupHookMu.Unlock()

	if startupHookH != 0 {
		procUnhookWindowsHookEx.Call(startupHookH)
		startupHookH = 0
	}
	x, y, w, h := ComputeInitialWindowRect(savedX, savedY, logicalW, logicalH)
	startupRect = rect{Left: x, Top: y, Right: x + w, Bottom: y + h}
	startupDark = dark
	startupTarget = 0

	tid, _, _ := procGetCurrentThreadId.Call()
	hHook, _, _ := procSetWindowsHookExW.Call(5 /* WH_CBT */, cbtHookCB, 0, tid)
	startupHookH = hHook
}

func unhookStartupHook() {
	startupHookMu.Lock()
	if startupHookH != 0 {
		procUnhookWindowsHookEx.Call(startupHookH)
		startupHookH = 0
	}
	target := startupTarget
	orig := startupOrigProc
	startupTarget = 0
	startupOrigProc = 0
	startupHookMu.Unlock()

	if target != 0 && orig != 0 && getWindowLongPtr(target, gwlpWndProc) == startupWndProcCB {
		setWindowLongPtr(target, gwlpWndProc, orig)
	}
}

func startupPreShowWndProc(hwnd, msg, wParam, lParam uintptr) uintptr {
	startupHookMu.Lock()
	target := startupTarget
	orig := startupOrigProc
	isDark := startupDark
	startupHookMu.Unlock()

	if hwnd == target {
		switch msg {
		case wmShowWindow, wmWindowPosChanging:
			exStyle := getWindowLongPtr(hwnd, gwlExStyle)
			if (exStyle & wsExLayered) == 0 {
				setWindowLongPtr(hwnd, gwlExStyle, exStyle|wsExLayered)
			}
			procSetLayeredWindowAttributes.Call(hwnd, 0, 0, lwaAlpha)
			setWindowCloak(hwnd, true)
		case wmEraseBkgnd:
			fillWindowBg(hwnd, wParam, isDark)
			return 1
		}
	}

	if orig != 0 {
		ret, _, _ := procCallWindowProcW.Call(orig, hwnd, msg, wParam, lParam)
		return ret
	}
	return 0
}

func startupCBTHookProc(nCode int32, wParam uintptr, lParam unsafe.Pointer) uintptr {
	startupHookMu.Lock()
	hHook := startupHookH
	isDark := startupDark
	initRect := startupRect
	target := startupTarget
	startupHookMu.Unlock()

	if nCode == 3 /* HCBT_CREATEWND */ && wParam != 0 && lParam != nil && target == 0 {
		var buf [64]uint16
		n, _, _ := procGetClassNameW.Call(wParam, uintptr(unsafe.Pointer(&buf[0])), uintptr(len(buf)))
		if n > 0 && windows.UTF16ToString(buf[:n]) == "webview" {
			cbt := (*cbtCreateWndW)(lParam)
			if cbt != nil && cbt.Lpcs != nil && cbt.Lpcs.HwndParent == 0 {
				cbt.Lpcs.X = initRect.Left
				cbt.Lpcs.Y = initRect.Top
				cbt.Lpcs.Cx = initRect.Right - initRect.Left
				cbt.Lpcs.Cy = initRect.Bottom - initRect.Top

				style := getWindowLongPtr(wParam, gwlStyle)
				setWindowLongPtr(wParam, gwlStyle, style&^(wsCaption|wsThickFrame|wsMaximizeBox))

				// Subclass immediately so WM_SHOWWINDOW/WM_WINDOWPOSCHANGING applies WS_EX_LAYERED
				// with alpha=0 after CreateWindowExW finishes and before ShowWindow draws a pixel.
				orig := setWindowLongPtr(wParam, gwlpWndProc, startupWndProcCB)

				startupHookMu.Lock()
				startupTarget = wParam
				startupOrigProc = orig
				if startupHookH != 0 {
					procUnhookWindowsHookEx.Call(startupHookH)
					startupHookH = 0
				}
				startupHookMu.Unlock()

				if procDwmSetWindowAttribute.Find() == nil {
					val := uint32(0)
					if isDark {
						val = 1
					}
					procDwmSetWindowAttribute.Call(wParam, dwmwaUseImmersiveDarkMode, uintptr(unsafe.Pointer(&val)), unsafe.Sizeof(val))
					round := uint32(dwmwcpRound)
					procDwmSetWindowAttribute.Call(wParam, dwmwaWindowCornerPreference, uintptr(unsafe.Pointer(&round)), unsafe.Sizeof(round))
				}
				if procDwmExtendFrameIntoClientArea.Find() == nil {
					m := margins{0, 0, 1, 0}
					procDwmExtendFrameIntoClientArea.Call(wParam, uintptr(unsafe.Pointer(&m)))
				}
				setWindowCloak(wParam, true)
			}
		}
	}

	ret, _, _ := procCallNextHookEx.Call(hHook, uintptr(nCode), wParam, uintptr(lParam))
	return ret
}

// NewController attaches Win32 non-resizable styling, Windows 11 DWM theme, tray icon, and WndProc hooks.
func NewController(hwnd unsafe.Pointer, cb Callbacks) *Controller {
	return newController(uintptr(hwnd), cb)
}

func newController(h uintptr, cb Callbacks) *Controller {
	startupHookMu.Lock()
	isDark := startupDark
	startupHookMu.Unlock()
	unhookStartupHook()

	c := &Controller{
		hwnd:      h,
		isDark:    isDark,
		callbacks: cb,
	}

	ctrlMu.Lock()
	controllers[h] = c
	ctrlMu.Unlock()

	c.applyWindowChrome()
	c.initTray()
	c.registerMediaHotKeys()

	orig := setWindowLongPtr(h, gwlpWndProc, wndProcCB)
	c.origWndProc = orig

	var rc rect
	procGetWindowRect.Call(h, uintptr(unsafe.Pointer(&rc)))
	c.savedRect = rc

	return c
}

func (c *Controller) applyWindowChrome() {
	// Ensure window is strictly non-resizable with layered alpha preserved until RevealWindow.
	style := getWindowLongPtr(c.hwnd, gwlStyle)
	style &^= (wsThickFrame | wsMaximizeBox)
	setWindowLongPtr(c.hwnd, gwlStyle, style)
	procSetWindowPos.Call(c.hwnd, 0, 0, 0, 0, 0, swpNoMove|swpNoSize|swpNoZOrder|swpNoActivate|swpFrameChanged)

	if procDwmSetWindowAttribute.Find() == nil {
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

	titleStrip := rect{
		Left:   int32(x),
		Top:    int32(y),
		Right:  int32(x + min(w, 120)),
		Bottom: int32(y + 40),
	}
	if (x == -1 && y == -1) || x < minX || y < minY || x > maxX || y > maxY || !isRectOnAnyMonitor(titleStrip) {
		x = int(wa.Left) + (int(wa.Right-wa.Left)-w)/2
		y = int(wa.Top) + (int(wa.Bottom-wa.Top)-h)/2
	}

	procSetWindowPos.Call(c.hwnd, 0, uintptr(x), uintptr(y), 0, 0, swpNoSize|swpNoZOrder|swpNoActivate)
	procGetWindowRect.Call(c.hwnd, uintptr(unsafe.Pointer(&rc)))
	c.savedRect = rc
}

// GetWindowPosition returns the last known top-left screen coordinates of the window.
func (c *Controller) GetWindowPosition() (int, int) {
	if !c.hidden && c.trayAnim.mode == trayAnimNone && !c.resizeAnim.active {
		if isIconic, _, _ := procIsIconic.Call(c.hwnd); isIconic == 0 {
			var rc rect
			procGetWindowRect.Call(c.hwnd, uintptr(unsafe.Pointer(&rc)))
			if (rc.Right-rc.Left) > 80 && (rc.Bottom-rc.Top) > 80 {
				c.savedRect = rc
			}
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

	if c.hidden || c.trayAnim.mode != trayAnimNone {
		c.savedRect.Right = c.savedRect.Left + winW
		c.savedRect.Bottom = c.savedRect.Top + winH
		if c.hidden {
			procSetWindowPos.Call(c.hwnd, 0, 0, 0, uintptr(winW), uintptr(winH), swpNoMove|swpNoZOrder|swpNoActivate)
		}
		return
	}

	var cur rect
	procGetWindowRect.Call(c.hwnd, uintptr(unsafe.Pointer(&cur)))
	curH := cur.Bottom - cur.Top

	if !animate || curH == winH || (cur.Right-cur.Left) == 0 {
		if c.resizeAnim.active {
			c.resizeAnim.active = false
			procKillTimer.Call(c.hwnd, timerResizeID)
			if c.trayAnim.mode == trayAnimNone {
				c.endHighResTimer()
			}
		}
		procSetWindowPos.Call(c.hwnd, 0, 0, 0, uintptr(winW), uintptr(winH), swpNoMove|swpNoZOrder|swpNoActivate)
		procGetWindowRect.Call(c.hwnd, uintptr(unsafe.Pointer(&c.savedRect)))
		if c.callbacks.OnResizeNeeded != nil {
			c.callbacks.OnResizeNeeded()
		}
		return
	}

	c.beginHighResTimer()
	c.resizeAnim = resizeAnimState{
		active:    true,
		startTime: time.Now(),
		startH:    curH,
		targetH:   winH,
		winW:      winW,
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

// SetDarkMode switches the Windows 11 native title bar between dark and light mode.
func (c *Controller) SetDarkMode(dark bool) {
	c.isDark = dark
	if procDwmSetWindowAttribute.Find() == nil {
		val := uint32(0)
		if dark {
			val = 1
		}
		procDwmSetWindowAttribute.Call(c.hwnd, dwmwaUseImmersiveDarkMode, uintptr(unsafe.Pointer(&val)), unsafe.Sizeof(val))
	}
}

// RevealWindow restores full opacity after startup initialization and first UI frame paint complete.
func (c *Controller) RevealWindow() {
	if c.quitting || c.revealed {
		return
	}
	c.revealed = true
	if c.hidden || c.trayAnim.mode != trayAnimNone {
		setWindowCloak(c.hwnd, false)
		return
	}
	procSetLayeredWindowAttributes.Call(c.hwnd, 0, 255, lwaAlpha)
	exStyle := getWindowLongPtr(c.hwnd, gwlExStyle)
	if (exStyle & wsExLayered) != 0 {
		setWindowLongPtr(c.hwnd, gwlExStyle, exStyle&^wsExLayered)
	}
	// Uncloak after restoring opacity and clearing WS_EX_LAYERED so DWM presents the first fully composited frame atomically.
	setWindowCloak(c.hwnd, false)
	procSetForegroundWindow.Call(c.hwnd)
	c.SetAlwaysOnTop(c.alwaysOnTop)
}

// BeginQuit marks the window as exiting, cleans up tray icon, and triggers WM_CLOSE destruction.
func (c *Controller) BeginQuit() {
	c.quitting = true
	procKillTimer.Call(c.hwnd, timerResizeID)
	procKillTimer.Call(c.hwnd, timerTrayAnimID)
	c.endHighResTimer()
	c.unregisterMediaHotKeys()
	c.removeTray()
	procDestroyWindow.Call(c.hwnd)
}

func (c *Controller) registerMediaHotKeys() {
	procRegisterHotKey.Call(c.hwnd, hkPlayPause, 0, vkMediaPlayPause)
	procRegisterHotKey.Call(c.hwnd, hkNext, 0, vkMediaNextTrack)
	procRegisterHotKey.Call(c.hwnd, hkPrev, 0, vkMediaPrevTrack)
}

func (c *Controller) unregisterMediaHotKeys() {
	procUnregisterHotKey.Call(c.hwnd, hkPlayPause)
	procUnregisterHotKey.Call(c.hwnd, hkNext)
	procUnregisterHotKey.Call(c.hwnd, hkPrev)
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
		ret, _, _ := procDefWindowProcW.Call(hwnd, msg, wParam, lParam)
		return ret
	}

	switch msg {
	case wmEraseBkgnd:
		fillWindowBg(hwnd, wParam, c.isDark)
		return 1

	case wmClose:
		if !c.quitting {
			c.MinimizeToTray()
			return 0
		}
		c.removeTray()

	case wmQueryEndSession:
		return 1

	case wmEndSession:
		if wParam != 0 {
			c.quitting = true
			if c.callbacks.OnExit != nil {
				c.callbacks.OnExit()
			} else {
				c.BeginQuit()
			}
			return 0
		}

	case wmSysCommand:
		if (wParam & 0xFFF0) == scMinimize {
			c.MinimizeToTray()
			return 0
		}

	case wmHotKey:
		switch wParam {
		case hkPlayPause:
			if c.callbacks.OnPlayPause != nil {
				c.callbacks.OnPlayPause()
			}
			return 0
		case hkNext:
			if c.callbacks.OnNext != nil {
				c.callbacks.OnNext()
			}
			return 0
		case hkPrev:
			if c.callbacks.OnPrev != nil {
				c.callbacks.OnPrev()
			}
			return 0
		}

	case wmDpiChanged:
		if lParam != 0 {
			var suggested rect
			procCopyRect.Call(uintptr(unsafe.Pointer(&suggested)), lParam)
			w := suggested.Right - suggested.Left
			h := suggested.Bottom - suggested.Top
			if w > 80 && h > 80 {
				if c.resizeAnim.active {
					c.resizeAnim.active = false
					procKillTimer.Call(hwnd, timerResizeID)
					if c.trayAnim.mode == trayAnimNone {
						c.endHighResTimer()
					}
				}
				if c.trayAnim.mode == trayAnimNone {
					procSetWindowPos.Call(hwnd, 0, uintptr(suggested.Left), uintptr(suggested.Top), uintptr(w), uintptr(h), swpNoZOrder|swpNoActivate)
					c.savedRect = suggested
					if c.callbacks.OnResizeNeeded != nil {
						c.callbacks.OnResizeNeeded()
					}
				} else {
					c.savedRect.Right = c.savedRect.Left + w
					c.savedRect.Bottom = c.savedRect.Top + h
				}
			}
			return 0
		}

	case wmTrayIcon:
		evt := lParam & 0xFFFF
		switch evt {
		case wmLButtonUp:
			c.ToggleTrayVisibility()
			return 0
		case wmMButtonUp:
			if c.callbacks.OnPlayPause != nil {
				c.callbacks.OnPlayPause()
			}
			return 0
		case wmRButtonUp, wmContextMenu:
			c.showTrayMenu()
			return 0
		}

	case wmPowerBroadcast:
		switch wParam {
		case pbtApmSuspend:
			if c.callbacks.OnSystemSuspend != nil {
				c.callbacks.OnSystemSuspend()
			}
		case pbtApmResumeAuto:
			if c.callbacks.OnSystemResume != nil {
				c.callbacks.OnSystemResume()
			}
		}
		return 1

	case wmRestoreApp:
		c.RestoreFromTray()
		return 0

	case wmResetStats:
		if c.callbacks.OnResetStats != nil {
			c.callbacks.OnResetStats()
		}
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
		case cmdToggleTheme:
			if c.callbacks.OnToggleTheme != nil {
				c.callbacks.OnToggleTheme()
			}
			return 0
		case cmdToggleShuffle:
			if c.callbacks.OnToggleShuffle != nil {
				c.callbacks.OnToggleShuffle()
			}
			return 0
		case cmdCycleRepeat:
			if c.callbacks.OnCycleRepeat != nil {
				c.callbacks.OnCycleRepeat()
			}
			return 0
		case cmdSleepOff:
			if c.callbacks.OnSetSleepTimer != nil {
				c.callbacks.OnSetSleepTimer(0)
			}
			return 0
		case cmdSleep15:
			if c.callbacks.OnSetSleepTimer != nil {
				c.callbacks.OnSetSleepTimer(15)
			}
			return 0
		case cmdSleep30:
			if c.callbacks.OnSetSleepTimer != nil {
				c.callbacks.OnSetSleepTimer(30)
			}
			return 0
		case cmdSleep60:
			if c.callbacks.OnSetSleepTimer != nil {
				c.callbacks.OnSetSleepTimer(60)
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
		c.endHighResTimer()
		c.unregisterMediaHotKeys()
		c.removeTray()
		orig := c.origWndProc
		if orig != 0 {
			setWindowLongPtr(hwnd, gwlpWndProc, orig)
		}
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

// OpenExternalURL opens an http/https URL using the default Windows shell browser.
func OpenExternalURL(rawURL string) error {
	trimmed := strings.TrimSpace(rawURL)
	lower := strings.ToLower(trimmed)
	if !strings.HasPrefix(lower, "https://") && !strings.HasPrefix(lower, "http://") {
		return fmt.Errorf("unsupported URL scheme")
	}
	verb, _ := windows.UTF16PtrFromString("open")
	target, err := windows.UTF16PtrFromString(trimmed)
	if err != nil {
		return err
	}
	ret, _, _ := procShellExecuteW.Call(0, uintptr(unsafe.Pointer(verb)), uintptr(unsafe.Pointer(target)), 0, 0, 1 /* SW_SHOWNORMAL */)
	if ret <= 32 {
		return fmt.Errorf("ShellExecuteW failed with code %d", ret)
	}
	return nil
}
