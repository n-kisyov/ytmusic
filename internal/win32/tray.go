//go:build windows

package win32

import (
	"fmt"
	"math"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

const (
	nifMessage = 0x00000001
	nifIcon    = 0x00000002
	nifTip     = 0x00000004

	nimAdd    = 0x00000000
	nimModify = 0x00000001
	nimDelete = 0x00000002

	mfString    = 0x00000000
	mfChecked   = 0x00000008
	mfPopup     = 0x00000010
	mfSeparator = 0x00000800

	tpmLeftAlign   = 0x0000
	tpmBottomAlign = 0x0020
	tpmRightButton = 0x0002
)

const (
	cmdOpen              = 1001
	cmdPlayPause         = 1002
	cmdNext              = 1003
	cmdPrev              = 1004
	cmdToggleVideo       = 1005
	cmdExit              = 1006
	cmdToggleAlwaysOnTop = 1007
	cmdToggleShuffle     = 1008
	cmdCycleRepeat       = 1009
	cmdSleepOff          = 1010
	cmdSleep15           = 1011
	cmdSleep30           = 1012
	cmdSleep60           = 1013
)

type guid struct {
	Data1 uint32
	Data2 uint16
	Data3 uint16
	Data4 [8]byte
}

type notifyIconDataW struct {
	CbSize           uint32
	Hwnd             uintptr
	UID              uint32
	UFlags           uint32
	UCallbackMessage uint32
	HIcon            uintptr
	SzTip            [128]uint16
	DwState          uint32
	DwStateMask      uint32
	SzInfo           [256]uint16
	UVersion         uint32
	SzInfoTitle      [64]uint16
	DwInfoFlags      uint32
	GuidItem         guid
	HBalloonIcon     uintptr
}

type notifyIconIdentifier struct {
	CbSize   uint32
	Hwnd     uintptr
	UID      uint32
	GuidItem guid
}

type iconInfo struct {
	FIcon    int32
	XHotspot uint32
	YHotspot uint32
	HbmMask  uintptr
	HbmColor uintptr
}

type bitmapInfoHeader struct {
	BiSize          uint32
	BiWidth         int32
	BiHeight        int32
	BiPlanes        uint16
	BiBitCount      uint16
	BiCompression   uint32
	BiSizeImage     uint32
	BiXPelsPerMeter int32
	BiYPelsPerMeter int32
	BiClrUsed       uint32
	BiClrImportant  uint32
}

type bitmapInfo struct {
	BmiHeader bitmapInfoHeader
	BmiColors [1]uint32
}

var (
	shell32                     = windows.NewLazySystemDLL("shell32.dll")
	procShellNotifyIconW        = shell32.NewProc("Shell_NotifyIconW")
	procShellNotifyIconGetRect  = shell32.NewProc("Shell_NotifyIconGetRect")
	gdi32                       = windows.NewLazySystemDLL("gdi32.dll")
	procCreateDIBSection        = gdi32.NewProc("CreateDIBSection")
	procCreateBitmap            = gdi32.NewProc("CreateBitmap")
	procDeleteObject            = gdi32.NewProc("DeleteObject")
	procCreateIconIndirect      = user32.NewProc("CreateIconIndirect")
	procDestroyIcon             = user32.NewProc("DestroyIcon")
	procCreatePopupMenu         = user32.NewProc("CreatePopupMenu")
	procAppendMenuW             = user32.NewProc("AppendMenuW")
	procSetMenuDefaultItem      = user32.NewProc("SetMenuDefaultItem")
	procTrackPopupMenu          = user32.NewProc("TrackPopupMenu")
	procDestroyMenu             = user32.NewProc("DestroyMenu")
	procGetCursorPos            = user32.NewProc("GetCursorPos")
	procSetForegroundWindow     = user32.NewProc("SetForegroundWindow")
)

func (c *Controller) initTray() {
	c.hIcon = createAppIcon(32)
	if c.hIcon != 0 {
		// WM_SETICON = 0x0080 (ICON_SMALL=0, ICON_BIG=1)
		procSendMessageW.Call(c.hwnd, 0x0080, 0, c.hIcon)
		procSendMessageW.Call(c.hwnd, 0x0080, 1, c.hIcon)
	}

	var nid notifyIconDataW
	nid.CbSize = uint32(unsafe.Sizeof(nid))
	nid.Hwnd = c.hwnd
	nid.UID = 1
	nid.UFlags = nifMessage | nifIcon | nifTip
	nid.UCallbackMessage = wmTrayIcon
	nid.HIcon = c.hIcon
	tip, _ := windows.UTF16FromString("ytmusic")
	copy(nid.SzTip[:], tip)

	r, _, _ := procShellNotifyIconW.Call(nimAdd, uintptr(unsafe.Pointer(&nid)))
	c.trayAdded = r != 0
}

// UpdateTrayTooltip updates the hover tooltip on the tray icon.
func (c *Controller) UpdateTrayTooltip(tip string) {
	if !c.trayAdded {
		return
	}
	if len(tip) > 120 {
		tip = tip[:117] + "..."
	}
	var nid notifyIconDataW
	nid.CbSize = uint32(unsafe.Sizeof(nid))
	nid.Hwnd = c.hwnd
	nid.UID = 1
	nid.UFlags = nifTip
	utf16Tip, _ := windows.UTF16FromString(tip)
	copy(nid.SzTip[:], utf16Tip)
	procShellNotifyIconW.Call(nimModify, uintptr(unsafe.Pointer(&nid)))
}

func (c *Controller) removeTray() {
	if c.trayAdded {
		var nid notifyIconDataW
		nid.CbSize = uint32(unsafe.Sizeof(nid))
		nid.Hwnd = c.hwnd
		nid.UID = 1
		procShellNotifyIconW.Call(nimDelete, uintptr(unsafe.Pointer(&nid)))
		c.trayAdded = false
	}
	if c.hIcon != 0 {
		procDestroyIcon.Call(c.hIcon)
		c.hIcon = 0
	}
}

func (c *Controller) showTrayMenu() {
	hMenu, _, _ := procCreatePopupMenu.Call()
	if hMenu == 0 {
		return
	}
	defer procDestroyMenu.Call(hMenu)

	appendItem := func(flags uintptr, id uintptr, text string) {
		var ptr uintptr
		if text != "" {
			u, _ := windows.UTF16PtrFromString(text)
			ptr = uintptr(unsafe.Pointer(u))
		}
		procAppendMenuW.Call(hMenu, flags, id, ptr)
	}

	openLabel := "Open ytmusic"
	if !c.hidden {
		openLabel = "Minimize to Tray"
	}
	appendItem(mfString, cmdOpen, openLabel)
	procSetMenuDefaultItem.Call(hMenu, cmdOpen, 0)
	appendItem(mfSeparator, 0, "")
	appendItem(mfString, cmdPlayPause, "Play / Pause")
	appendItem(mfString, cmdNext, "Next Track")
	appendItem(mfString, cmdPrev, "Previous Track")
	appendItem(mfSeparator, 0, "")

	shuffleFlags := uintptr(mfString)
	if c.callbacks.IsShuffleEnabled != nil && c.callbacks.IsShuffleEnabled() {
		shuffleFlags |= mfChecked
	}
	appendItem(shuffleFlags, cmdToggleShuffle, "Shuffle")

	repeatMode := "off"
	if c.callbacks.GetRepeatMode != nil {
		repeatMode = c.callbacks.GetRepeatMode()
	}
	repeatLabel := "Repeat: Off"
	repeatFlags := uintptr(mfString)
	switch repeatMode {
	case "all":
		repeatLabel = "Repeat: All"
		repeatFlags |= mfChecked
	case "one":
		repeatLabel = "Repeat: One"
		repeatFlags |= mfChecked
	}
	appendItem(repeatFlags, cmdCycleRepeat, repeatLabel)

	appendItem(mfSeparator, 0, "")

	videoFlags := uintptr(mfString)
	if c.callbacks.IsVideoEnabled != nil && c.callbacks.IsVideoEnabled() {
		videoFlags |= mfChecked
	}
	appendItem(videoFlags, cmdToggleVideo, "Video Playback")

	topFlags := uintptr(mfString)
	if c.callbacks.IsAlwaysOnTop != nil && c.callbacks.IsAlwaysOnTop() {
		topFlags |= mfChecked
	}
	appendItem(topFlags, cmdToggleAlwaysOnTop, "Always on Top")

	sleepMin := 0
	if c.callbacks.GetSleepTimerMinutes != nil {
		sleepMin = c.callbacks.GetSleepTimerMinutes()
	}
	hSleepMenu, _, _ := procCreatePopupMenu.Call()
	if hSleepMenu != 0 {
		appendSleep := func(mins int, id uintptr, label string) {
			flags := uintptr(mfString)
			if sleepMin == mins {
				flags |= mfChecked
			}
			u, _ := windows.UTF16PtrFromString(label)
			procAppendMenuW.Call(hSleepMenu, flags, id, uintptr(unsafe.Pointer(u)))
		}
		appendSleep(0, cmdSleepOff, "Off")
		appendSleep(15, cmdSleep15, "15 minutes")
		appendSleep(30, cmdSleep30, "30 minutes")
		appendSleep(60, cmdSleep60, "60 minutes")

		sleepLabel := "Sleep Timer"
		if sleepMin > 0 {
			sleepLabel = fmt.Sprintf("Sleep Timer (%dm)", sleepMin)
		}
		uSleep, _ := windows.UTF16PtrFromString(sleepLabel)
		procAppendMenuW.Call(hMenu, mfPopup, hSleepMenu, uintptr(unsafe.Pointer(uSleep)))
	}

	appendItem(mfSeparator, 0, "")
	appendItem(mfString, cmdExit, "Exit")

	var pt point
	procGetCursorPos.Call(uintptr(unsafe.Pointer(&pt)))
	procSetForegroundWindow.Call(c.hwnd)
	procTrackPopupMenu.Call(hMenu, tpmLeftAlign|tpmBottomAlign|tpmRightButton, uintptr(pt.X), uintptr(pt.Y), 0, c.hwnd, 0)
	// Required by Win32 so the tray menu dismisses cleanly on outside clicks.
	procPostMessageW.Call(c.hwnd, 0, 0, 0)
}

func (c *Controller) getTrayIconRect() rect {
	nii := notifyIconIdentifier{
		CbSize: uint32(unsafe.Sizeof(notifyIconIdentifier{})),
		Hwnd:   c.hwnd,
		UID:    1,
	}
	var rc rect
	hr, _, _ := procShellNotifyIconGetRect.Call(uintptr(unsafe.Pointer(&nii)), uintptr(unsafe.Pointer(&rc)))
	if int32(hr) >= 0 && (rc.Right-rc.Left) > 0 && (rc.Bottom-rc.Top) > 0 {
		return rc
	}

	// Fallback: TrayNotifyWnd inside Shell_TrayWnd.
	shellClass, _ := windows.UTF16PtrFromString("Shell_TrayWnd")
	shellWnd, _, _ := procFindWindowW.Call(uintptr(unsafe.Pointer(shellClass)), 0)
	if shellWnd != 0 {
		trayClass, _ := windows.UTF16PtrFromString("TrayNotifyWnd")
		trayWnd, _, _ := procFindWindowExW.Call(shellWnd, 0, uintptr(unsafe.Pointer(trayClass)), 0)
		if trayWnd != 0 {
			var tr rect
			ok, _, _ := procGetWindowRect.Call(trayWnd, uintptr(unsafe.Pointer(&tr)))
			if ok != 0 {
				return rect{
					Left:   tr.Left + (tr.Right-tr.Left)/2 - 12,
					Top:    tr.Top + (tr.Bottom-tr.Top)/2 - 12,
					Right:  tr.Left + (tr.Right-tr.Left)/2 + 12,
					Bottom: tr.Top + (tr.Bottom-tr.Top)/2 + 12,
				}
			}
		}
	}

	// Final fallback: bottom-right corner of primary work area.
	var wa rect
	procSystemParametersInfoW.Call(48 /* SPI_GETWORKAREA */, 0, uintptr(unsafe.Pointer(&wa)), 0)
	return rect{
		Left:   wa.Right - 48,
		Top:    wa.Bottom - 24,
		Right:  wa.Right - 24,
		Bottom: wa.Bottom,
	}
}

// createAppIcon draws a 32x32 ARGB icon (dark rounded square with crimson badge & white play glyph).
func createAppIcon(size int) uintptr {
	bmi := bitmapInfo{
		BmiHeader: bitmapInfoHeader{
			BiSize:        uint32(unsafe.Sizeof(bitmapInfoHeader{})),
			BiWidth:       int32(size),
			BiHeight:      -int32(size), // top-down DIB
			BiPlanes:      1,
			BiBitCount:    32,
			BiCompression: 0, // BI_RGB
		},
	}

	var bits unsafe.Pointer
	hColor, _, _ := procCreateDIBSection.Call(0, uintptr(unsafe.Pointer(&bmi)), 0, uintptr(unsafe.Pointer(&bits)), 0, 0)
	if hColor == 0 || bits == nil {
		return 0
	}
	defer procDeleteObject.Call(hColor)

	pixels := unsafe.Slice((*uint32)(bits), size*size)
	cx := float64(size-1) / 2.0
	cy := float64(size-1) / 2.0
	half := float64(size) / 2.0
	corner := float64(size) * 0.24

	for y := 0; y < size; y++ {
		for x := 0; x < size; x++ {
			fx := float64(x) + 0.5
			fy := float64(y) + 0.5

			// Signed distance to rounded square.
			dx := math.Abs(fx-half) - (half - corner - 1.0)
			dy := math.Abs(fy-half) - (half - corner - 1.0)
			outDist := math.Hypot(math.Max(dx, 0), math.Max(dy, 0)) + math.Min(math.Max(dx, dy), 0) - corner

			if outDist > 0.7 {
				pixels[y*size+x] = 0x00000000
				continue
			}

			// Base dark obsidian background.
			bgAlpha := clampByte((0.7 - outDist) * 255.0)
			r, g, b := 24.0, 24.0, 32.0

			// Outer subtle rim highlight.
			if outDist > -1.3 {
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
			// Triangle vertices: (cx-3, cy-5), (cx-3, cy+5), (cx+5.5, cy)
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

			_ = cy
			pixels[y*size+x] = (uint32(bgAlpha) << 24) | (uint32(r) << 16) | (uint32(g) << 8) | uint32(b)
		}
	}

	// Empty 1bpp AND mask (alpha comes from 32-bit DIB).
	maskBytes := make([]byte, ((size+15)/16)*2*size)
	hMask, _, _ := procCreateBitmap.Call(uintptr(size), uintptr(size), 1, 1, uintptr(unsafe.Pointer(&maskBytes[0])))
	if hMask == 0 {
		return 0
	}
	defer procDeleteObject.Call(hMask)

	ii := iconInfo{
		FIcon:    1,
		HbmMask:  hMask,
		HbmColor: hColor,
	}
	hIcon, _, _ := procCreateIconIndirect.Call(uintptr(unsafe.Pointer(&ii)))
	return hIcon
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

var _ = syscall.Errno(0)
