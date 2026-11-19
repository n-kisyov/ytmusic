//go:build windows

package win32

import (
	"runtime"
	"strings"
	"testing"
	"unicode/utf16"
	"unicode/utf8"
	"unsafe"

	"golang.org/x/sys/windows"
)

func TestStartupHookAppliesLayeredExactRectAndStripsResizeBeforeShow(t *testing.T) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	procRegisterClassExW := user32.NewProc("RegisterClassExW")
	procCreateWindowExW := user32.NewProc("CreateWindowExW")
	procDefWindowProcW := user32.NewProc("DefWindowProcW")

	type wndClassExW struct {
		CbSize        uint32
		Style         uint32
		LpfnWndProc   uintptr
		CbClsExtra    int32
		CbWndExtra    int32
		HInstance     windows.Handle
		HIcon         uintptr
		HCursor       uintptr
		HbrBackground uintptr
		LpszMenuName  *uint16
		LpszClassName *uint16
		HIconSm       uintptr
	}

	var hInstance windows.Handle
	_ = windows.GetModuleHandleEx(0, nil, &hInstance)
	className, _ := windows.UTF16PtrFromString("webview")
	winTitle, _ := windows.UTF16PtrFromString("ytmusic_test")

	var exStyleDuringShow uintptr
	wndProc := windows.NewCallback(func(h, msg, wp, lp uintptr) uintptr {
		if msg == wmWindowPosChanging {
			exStyleDuringShow = getWindowLongPtr(h, gwlExStyle)
		}
		r, _, _ := procDefWindowProcW.Call(h, msg, wp, lp)
		return r
	})
	wc := wndClassExW{
		CbSize:        uint32(unsafe.Sizeof(wndClassExW{})),
		HInstance:     hInstance,
		LpszClassName: className,
		LpfnWndProc:   wndProc,
	}
	procRegisterClassExW.Call(uintptr(unsafe.Pointer(&wc)))

	wantX, wantY, wantW, wantH := ComputeInitialWindowRect(120, 90, 380, 482)
	InstallStartupHook(true, 120, 90, 380, 482)
	t.Logf("startupHookH=%v startupRect=%+v", startupHookH, startupRect)

	// Simulate go-webview2 passing CW_USEDEFAULT / WS_OVERLAPPEDWINDOW / 0 exStyle.
	const cwUseDefault = 0x80000000
	hwnd, _, _ := procCreateWindowExW.Call(
		0,
		uintptr(unsafe.Pointer(className)),
		uintptr(unsafe.Pointer(winTitle)),
		0x00CF0000, // WS_OVERLAPPEDWINDOW
		cwUseDefault,
		cwUseDefault,
		640,
		480,
		0, 0, uintptr(hInstance), 0,
	)
	if hwnd == 0 {
		unhookStartupHook()
		t.Fatal("CreateWindowExW failed")
	}
	defer procDestroyWindow.Call(hwnd)

	var rc rect
	procGetWindowRect.Call(hwnd, uintptr(unsafe.Pointer(&rc)))
	t.Logf("after CreateWindowExW: exStyle=0x%x style=0x%x rect=(%d,%d,%dx%d) target=%v",
		getWindowLongPtr(hwnd, gwlExStyle), getWindowLongPtr(hwnd, gwlStyle),
		rc.Left, rc.Top, rc.Right-rc.Left, rc.Bottom-rc.Top, startupTarget)

	procShowWindow.Call(hwnd, 5 /* SW_SHOW */)

	if (exStyleDuringShow & wsExLayered) == 0 {
		t.Fatalf("expected WS_EX_LAYERED already set on WM_WINDOWPOSCHANGING before window becomes visible, got 0x%x", exStyleDuringShow)
	}
	exStyle := getWindowLongPtr(hwnd, gwlExStyle)
	if (exStyle & wsExLayered) == 0 {
		t.Fatalf("expected WS_EX_LAYERED applied during HCBT_CREATEWND, got exStyle=0x%x", exStyle)
	}
	style := getWindowLongPtr(hwnd, gwlStyle)
	if (style & (wsThickFrame | wsMaximizeBox)) != 0 {
		t.Fatalf("expected WS_THICKFRAME and WS_MAXIMIZEBOX stripped, got style=0x%x", style)
	}

	if rc.Left != wantX || rc.Top != wantY || (rc.Right-rc.Left) != wantW || (rc.Bottom-rc.Top) != wantH {
		t.Fatalf("expected rect (%d,%d,%dx%d), got (%d,%d,%dx%d)",
			wantX, wantY, wantW, wantH,
			rc.Left, rc.Top, rc.Right-rc.Left, rc.Bottom-rc.Top)
	}

	ctrl := newController(hwnd, Callbacks{})
	defer ctrl.removeTray()

	// Still layered (hidden) until RevealWindow is called.
	if (getWindowLongPtr(hwnd, gwlExStyle) & wsExLayered) == 0 {
		t.Fatalf("expected window to remain layered until RevealWindow")
	}

	ctrl.RevealWindow()
	if (getWindowLongPtr(hwnd, gwlExStyle) & wsExLayered) != 0 {
		t.Fatalf("expected WS_EX_LAYERED cleared after RevealWindow")
	}

	// Verify high-res timer toggling & glide vector math.
	ctrl.beginHighResTimer()
	if !ctrl.timerPeriodOn {
		t.Fatalf("expected timerPeriodOn=true after beginHighResTimer")
	}
	dx, dy := ctrl.trayGlideOffset(rect{Left: 100, Top: 100, Right: 480, Bottom: 580})
	if dx == 0 && dy == 0 {
		t.Fatalf("expected non-zero tray glide offset")
	}
	ctrl.endHighResTimer()
	if ctrl.timerPeriodOn {
		t.Fatalf("expected timerPeriodOn=false after endHighResTimer")
	}

	// Verify MinimizeToTray triggered mid-restore animation preserves resting savedRect.
	ctrl.savedRect = rect{Left: 200, Top: 150, Right: 580, Bottom: 630}
	ctrl.trayAnim.mode = trayAnimRestore
	ctrl.MinimizeToTray()
	procKillTimer.Call(ctrl.hwnd, timerTrayAnimID)
	ctrl.endHighResTimer()
	if ctrl.savedRect.Left != 200 || ctrl.savedRect.Top != 150 {
		t.Fatalf("expected savedRect preserved at (200,150) during mid-restore minimize, got (%d,%d)", ctrl.savedRect.Left, ctrl.savedRect.Top)
	}
}

func TestTruncateTooltipUTF8AndSurrogates(t *testing.T) {
	jp := strings.Repeat("初音ミク", 40) // 160 runes, > 400 UTF-8 bytes
	out := truncateTooltip(jp, 120)
	if !utf8.ValidString(out) {
		t.Fatalf("truncated tooltip produced invalid UTF-8: %q", out)
	}
	if units := len(utf16.Encode([]rune(out))); units > 120 {
		t.Fatalf("expected <= 120 UTF-16 code units, got %d", units)
	}
	if !strings.HasSuffix(out, "...") {
		t.Fatalf("expected ellipsis suffix, got %q", out)
	}

	emoji := strings.Repeat("🎵", 70) // 70 astral runes = 140 UTF-16 surrogate units
	outEmoji := truncateTooltip(emoji, 120)
	if !utf8.ValidString(outEmoji) {
		t.Fatalf("truncated emoji tooltip produced invalid UTF-8")
	}
	if units := len(utf16.Encode([]rune(outEmoji))); units > 120 {
		t.Fatalf("expected <= 120 UTF-16 code units for emoji, got %d", units)
	}
}
