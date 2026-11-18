//go:build windows

package auth

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"sync"
	"syscall"
	"time"
	"unsafe"

	"github.com/jchv/go-webview2/pkg/edge"
	"golang.org/x/sys/windows"

	"ytmusic/internal/config"
)

const youtubeLoginURL = "https://accounts.google.com/ServiceLogin?service=youtube&uilel=3&passive=true&continue=https%3A%2F%2Fwww.youtube.com%2Fsignin%3Faction_handle_signin%3Dtrue%26app%3Ddesktop%26hl%3Den%26next%3Dhttps%253A%252F%252Fwww.youtube.com%252F&hl=en"

var (
	user32                    = windows.NewLazySystemDLL("user32.dll")
	procRegisterClassExW      = user32.NewProc("RegisterClassExW")
	procCreateWindowExW       = user32.NewProc("CreateWindowExW")
	procDefWindowProcW        = user32.NewProc("DefWindowProcW")
	procShowWindow            = user32.NewProc("ShowWindow")
	procUpdateWindow          = user32.NewProc("UpdateWindow")
	procGetMessageW           = user32.NewProc("GetMessageW")
	procTranslateMessage      = user32.NewProc("TranslateMessage")
	procDispatchMessageW      = user32.NewProc("DispatchMessageW")
	procPostQuitMessage       = user32.NewProc("PostQuitMessage")
	procDestroyWindow         = user32.NewProc("DestroyWindow")
	procGetSystemMetrics      = user32.NewProc("GetSystemMetrics")
	dwmapi                    = windows.NewLazySystemDLL("dwmapi.dll")
	procDwmSetWindowAttribute = dwmapi.NewProc("DwmSetWindowAttribute")
)

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

type msgStruct struct {
	Hwnd    uintptr
	Message uint32
	WParam  uintptr
	LParam  uintptr
	Time    uint32
	Pt      struct{ X, Y int32 }
}

// LaunchLoginHelper spawns a child instance of this executable with --login and waits for completion.
func LaunchLoginHelper() (config.AuthState, error) {
	exe, err := os.Executable()
	if err != nil {
		return config.AuthState{}, fmt.Errorf("resolve executable: %w", err)
	}
	cmd := exec.Command(exe, "--login")
	if err := cmd.Run(); err != nil {
		return config.AuthState{}, fmt.Errorf("login helper exited with error: %w", err)
	}
	return config.LoadAuthState()
}

// RunLoginWindow opens a dedicated WebView2 sign-in window for Google/YouTube authentication.
func RunLoginWindow() error {
	dataDir, err := config.AuthWebViewProfileDir()
	if err != nil {
		return err
	}
	_ = os.MkdirAll(dataDir, 0o755)

	var hInstance windows.Handle
	_ = windows.GetModuleHandleEx(0, nil, &hInstance)

	chromium := edge.NewChromium()
	chromium.DataPath = dataDir

	var hwnd uintptr
	var doneOnce sync.Once
	var accountName string
	var webviewPtr *edge.ICoreWebView2

	finishLogin := func(cookies map[string]string, entries []config.CookieEntry, name string) {
		if len(cookies) == 0 {
			return
		}
		if cookies["SAPISID"] == "" && cookies["__Secure-3PAPISID"] == "" && cookies["SID"] == "" {
			return
		}
		if name == "" {
			name = "YouTube Account"
		}
		doneOnce.Do(func() {
			state := config.AuthState{
				LoggedIn:      true,
				AccountName:   name,
				Cookies:       cookies,
				CookieEntries: entries,
				UpdatedAt:     time.Now().Unix(),
			}
			_ = config.SaveAuthState(state)
			if hwnd != 0 {
				procDestroyWindow.Call(hwnd)
			}
		})
	}

	chromium.MessageCallback = func(raw string) {
		var msg struct {
			Type        string `json:"type"`
			LoggedIn    bool   `json:"logged_in"`
			AccountName string `json:"account_name"`
			Cookies     string `json:"cookies"`
		}
		if err := json.Unmarshal([]byte(raw), &msg); err != nil {
			return
		}
		if msg.AccountName != "" {
			accountName = msg.AccountName
		}
		if !msg.LoggedIn {
			return
		}

		// First attempt full WebView2 CookieManager extraction (captures HttpOnly SID / __Secure-* cookies).
		if webviewPtr != nil {
			extractAllCookies(webviewPtr, func(cookies map[string]string, entries []config.CookieEntry) {
				if len(cookies) > 0 {
					finishLogin(cookies, entries, accountName)
					return
				}
				// Fallback to document.cookie pairs if CookieManager returned nothing.
				parsed := parseDocumentCookie(msg.Cookies)
				finishLogin(parsed, nil, accountName)
			})
			return
		}
		parsed := parseDocumentCookie(msg.Cookies)
		finishLogin(parsed, nil, accountName)
	}

	chromium.NavigationCompletedCallback = func(sender *edge.ICoreWebView2, _ *edge.ICoreWebView2NavigationCompletedEventArgs) {
		webviewPtr = sender
		src := getWebViewSource(sender)
		if strings.Contains(src, "youtube.com") {
			chromium.Eval(`(function(){
				try {
					var cfg = (window.ytcfg && window.ytcfg.data_) || {};
					var loggedIn = Boolean(cfg.LOGGED_IN) || document.cookie.indexOf('SAPISID=') !== -1;
					var name = cfg.USER_DISPLAY_NAME || '';
					if (!name) {
						var el = document.querySelector('#account-name, yt-formatted-string#account-name');
						if (el && el.textContent) name = el.textContent.trim();
					}
					window.chrome.webview.postMessage(JSON.stringify({
						type: 'auth_check',
						logged_in: loggedIn,
						account_name: name,
						cookies: document.cookie || ''
					}));
				} catch (e) {}
			})();`)
		}
	}

	wndProc := windows.NewCallback(func(h, msg, wp, lp uintptr) uintptr {
		switch msg {
		case 0x0005: // WM_SIZE
			chromium.Resize()
			return 0
		case 0x0046, 0x0003: // WM_MOVE
			_ = chromium.NotifyParentWindowPositionChanged()
		case 0x0010: // WM_CLOSE
			procDestroyWindow.Call(h)
			return 0
		case 0x0002: // WM_DESTROY
			procPostQuitMessage.Call(0)
			return 0
		}
		r, _, _ := procDefWindowProcW.Call(h, msg, wp, lp)
		return r
	})

	className, _ := windows.UTF16PtrFromString("YTMusicAuthWindow")
	title, _ := windows.UTF16PtrFromString("Sign in with Google — ytmusic")

	wc := wndClassExW{
		CbSize:        uint32(unsafe.Sizeof(wndClassExW{})),
		HInstance:     hInstance,
		LpszClassName: className,
		LpfnWndProc:   wndProc,
	}
	procRegisterClassExW.Call(uintptr(unsafe.Pointer(&wc)))

	sw, _, _ := procGetSystemMetrics.Call(0)
	sh, _, _ := procGetSystemMetrics.Call(1)
	winW, winH := uintptr(480), uintptr(660)
	posX := (sw - winW) / 2
	posY := (sh - winH) / 2

	const loginWinStyle = uintptr(0x00CF0000 &^ (0x00040000 | 0x00010000))
	hwnd, _, _ = procCreateWindowExW.Call(
		0,
		uintptr(unsafe.Pointer(className)),
		uintptr(unsafe.Pointer(title)),
		loginWinStyle, // non-resizable overlapped window
		posX, posY, winW, winH,
		0, 0, uintptr(hInstance), 0,
	)
	if hwnd == 0 {
		return fmt.Errorf("failed to create login window")
	}

	if procDwmSetWindowAttribute.Find() == nil {
		dark := uint32(1)
		procDwmSetWindowAttribute.Call(hwnd, 20, uintptr(unsafe.Pointer(&dark)), unsafe.Sizeof(dark))
		round := uint32(2)
		procDwmSetWindowAttribute.Call(hwnd, 33, uintptr(unsafe.Pointer(&round)), unsafe.Sizeof(round))
	}

	procShowWindow.Call(hwnd, 5)
	procUpdateWindow.Call(hwnd)

	if !chromium.Embed(hwnd) {
		return fmt.Errorf("failed to embed WebView2 for login")
	}
	chromium.Resize()

	// Inject periodic watcher on youtube.com pages so sign-in finishes right after redirect.
	chromium.Init(`(function(){
		window.addEventListener('DOMContentLoaded', function() {
			if (!location.hostname.endsWith('youtube.com')) return;
			var checks = 0;
			var timer = setInterval(function() {
				checks++;
				var cfg = (window.ytcfg && window.ytcfg.data_) || {};
				var loggedIn = Boolean(cfg.LOGGED_IN) || document.cookie.indexOf('SAPISID=') !== -1;
				if (loggedIn || checks > 40) {
					if (loggedIn) {
						clearInterval(timer);
						var name = cfg.USER_DISPLAY_NAME || '';
						window.chrome.webview.postMessage(JSON.stringify({
							type: 'auth_check',
							logged_in: true,
							account_name: name,
							cookies: document.cookie || ''
						}));
					}
				}
			}, 400);
		});
	})();`)

	chromium.Navigate(youtubeLoginURL)

	var m msgStruct
	for {
		ret, _, _ := procGetMessageW.Call(uintptr(unsafe.Pointer(&m)), 0, 0, 0)
		if int32(ret) <= 0 {
			break
		}
		procTranslateMessage.Call(uintptr(unsafe.Pointer(&m)))
		procDispatchMessageW.Call(uintptr(unsafe.Pointer(&m)))
	}
	return nil
}

func parseDocumentCookie(raw string) map[string]string {
	out := make(map[string]string)
	for _, part := range strings.Split(raw, ";") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		idx := strings.IndexByte(part, '=')
		if idx <= 0 {
			continue
		}
		out[strings.TrimSpace(part[:idx])] = strings.TrimSpace(part[idx+1:])
	}
	return out
}

// --- COM Helpers for ICoreWebView2_2::GetCookieManager ---

type comObject struct {
	vtbl *[128]uintptr
}

func comVtblSlot(obj unsafe.Pointer, slot int) uintptr {
	return ((*comObject)(obj)).vtbl[slot]
}

func getWebViewSource(wv *edge.ICoreWebView2) string {
	obj := unsafe.Pointer(wv)
	var ptr *uint16
	// slot 4 in ICoreWebView2 = GetSource(LPWSTR*)
	hr, _, _ := syscall.SyscallN(comVtblSlot(obj, 4), uintptr(obj), uintptr(unsafe.Pointer(&ptr)))
	if int32(hr) < 0 || ptr == nil {
		return ""
	}
	defer windows.CoTaskMemFree(unsafe.Pointer(ptr))
	return windows.UTF16PtrToString(ptr)
}

type getCookiesHandlerVtbl struct {
	QueryInterface uintptr
	AddRef         uintptr
	Release        uintptr
	Invoke         uintptr
}

type getCookiesHandler struct {
	vtbl     *getCookiesHandlerVtbl
	callback func(res uintptr, list unsafe.Pointer)
}

var (
	cookieHandlerMu    sync.Mutex
	cookieHandlerStore = map[uintptr]*getCookiesHandler{}
	cookieHandlerVtbl  = getCookiesHandlerVtbl{
		QueryInterface: windows.NewCallback(func(this, riid uintptr, ppv *uintptr) uintptr {
			if ppv != nil {
				*ppv = this
			}
			return 0
		}),
		AddRef: windows.NewCallback(func(this uintptr) uintptr {
			return 1
		}),
		Release: windows.NewCallback(func(this uintptr) uintptr {
			return 1
		}),
		Invoke: windows.NewCallback(func(this, res uintptr, cookieList unsafe.Pointer) uintptr {
			cookieHandlerMu.Lock()
			h := cookieHandlerStore[this]
			delete(cookieHandlerStore, this)
			cookieHandlerMu.Unlock()
			if h != nil && h.callback != nil {
				h.callback(res, cookieList)
			}
			return 0
		}),
	}
)

func extractAllCookies(wv *edge.ICoreWebView2, done func(cookies map[string]string, entries []config.CookieEntry)) {
	obj := unsafe.Pointer(wv)
	iid2 := edge.NewGUID("{9E8F0CF8-E670-4B5E-B2BC-73E061E3184C}")
	var wv2 unsafe.Pointer
	// slot 0 = QueryInterface
	hr, _, _ := syscall.SyscallN(comVtblSlot(obj, 0), uintptr(obj), uintptr(unsafe.Pointer(iid2)), uintptr(unsafe.Pointer(&wv2)))
	if int32(hr) < 0 || wv2 == nil {
		done(nil, nil)
		return
	}
	defer syscall.SyscallN(comVtblSlot(wv2, 2), uintptr(wv2)) // Release ICoreWebView2_2

	var cm unsafe.Pointer
	// slot 66 on ICoreWebView2_2 = GetCookieManager
	hr, _, _ = syscall.SyscallN(comVtblSlot(wv2, 66), uintptr(wv2), uintptr(unsafe.Pointer(&cm)))
	if int32(hr) < 0 || cm == nil {
		done(nil, nil)
		return
	}

	handler := &getCookiesHandler{vtbl: &cookieHandlerVtbl}
	handlerPtr := uintptr(unsafe.Pointer(handler))
	handler.callback = func(res uintptr, list unsafe.Pointer) {
		defer syscall.SyscallN(comVtblSlot(cm, 2), uintptr(cm)) // Release CookieManager
		if int32(res) < 0 || list == nil {
			done(nil, nil)
			return
		}
		var count uint32
		// ICoreWebView2CookieList: slot 3 = get_Count
		syscall.SyscallN(comVtblSlot(list, 3), uintptr(list), uintptr(unsafe.Pointer(&count)))
		cookies := make(map[string]string, count)
		entries := make([]config.CookieEntry, 0, count)

		for i := uint32(0); i < count; i++ {
			var ck unsafe.Pointer
			// slot 4 = GetValueAtIndex
			hrItem, _, _ := syscall.SyscallN(comVtblSlot(list, 4), uintptr(list), uintptr(i), uintptr(unsafe.Pointer(&ck)))
			if int32(hrItem) < 0 || ck == nil {
				continue
			}

			readStr := func(slot int) string {
				var p *uint16
				hrS, _, _ := syscall.SyscallN(comVtblSlot(ck, slot), uintptr(ck), uintptr(unsafe.Pointer(&p)))
				if int32(hrS) < 0 || p == nil {
					return ""
				}
				defer windows.CoTaskMemFree(unsafe.Pointer(p))
				return windows.UTF16PtrToString(p)
			}

			name := readStr(3)   // get_Name
			value := readStr(4)  // get_Value
			domain := readStr(6) // get_Domain
			cpath := readStr(7)  // get_Path

			var exp float64
			syscall.SyscallN(comVtblSlot(ck, 8), uintptr(ck), uintptr(unsafe.Pointer(&exp))) // get_Expires
			var isHTTPOnly int32
			syscall.SyscallN(comVtblSlot(ck, 10), uintptr(ck), uintptr(unsafe.Pointer(&isHTTPOnly))) // get_IsHttpOnly
			var isSecure int32
			syscall.SyscallN(comVtblSlot(ck, 14), uintptr(ck), uintptr(unsafe.Pointer(&isSecure))) // get_IsSecure

			syscall.SyscallN(comVtblSlot(ck, 2), uintptr(ck)) // Release cookie

			if name != "" {
				cookies[name] = value
				entries = append(entries, config.CookieEntry{
					Name:     name,
					Value:    value,
					Domain:   domain,
					Path:     cpath,
					Expires:  int64(exp),
					Secure:   isSecure != 0,
					HTTPOnly: isHTTPOnly != 0,
				})
			}
		}
		done(cookies, entries)
	}

	cookieHandlerMu.Lock()
	cookieHandlerStore[handlerPtr] = handler
	cookieHandlerMu.Unlock()

	uri, _ := windows.UTF16PtrFromString("https://www.youtube.com")
	// ICoreWebView2CookieManager: slot 5 = GetCookies
	hr, _, _ = syscall.SyscallN(comVtblSlot(cm, 5), uintptr(cm), uintptr(unsafe.Pointer(uri)), handlerPtr)
	if int32(hr) < 0 {
		cookieHandlerMu.Lock()
		delete(cookieHandlerStore, handlerPtr)
		cookieHandlerMu.Unlock()
		syscall.SyscallN(comVtblSlot(cm, 2), uintptr(cm))
		done(nil, nil)
	}
}
