//go:build windows

package auth

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
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
	"ytmusic/internal/youtube"
)

const (
	youtubeLoginURL    = "https://accounts.google.com/ServiceLogin?service=youtube&uilel=3&continue=https%3A%2F%2Fwww.youtube.com%2Fsignin%3Faction_handle_signin%3Dtrue%26app%3Ddesktop%26hl%3Den%26next%3Dhttps%253A%252F%252Fwww.youtube.com%252F&hl=en"
	youtubeSwitcherURL = "https://www.youtube.com/channel_switcher?next=%2F&feature=masthead_switcher"

	wmSwitchChannel = 0x8010
	wmFinishLogin   = 0x8011
)

var (
	user32                    = windows.NewLazySystemDLL("user32.dll")
	procRegisterClassExW      = user32.NewProc("RegisterClassExW")
	procCreateWindowExW       = user32.NewProc("CreateWindowExW")
	procDefWindowProcW        = user32.NewProc("DefWindowProcW")
	procShowWindow            = user32.NewProc("ShowWindow")
	procUpdateWindow          = user32.NewProc("UpdateWindow")
	procSetWindowTextW        = user32.NewProc("SetWindowTextW")
	procGetMessageW           = user32.NewProc("GetMessageW")
	procTranslateMessage      = user32.NewProc("TranslateMessage")
	procDispatchMessageW      = user32.NewProc("DispatchMessageW")
	procPostMessageW          = user32.NewProc("PostMessageW")
	procPostQuitMessage       = user32.NewProc("PostQuitMessage")
	procDestroyWindow         = user32.NewProc("DestroyWindow")
	procGetSystemMetrics      = user32.NewProc("GetSystemMetrics")
	dwmapi                    = windows.NewLazySystemDLL("dwmapi.dll")
	procDwmSetWindowAttribute = dwmapi.NewProc("DwmSetWindowAttribute")
	ole32                     = windows.NewLazySystemDLL("ole32.dll")
	procCoInitializeEx        = ole32.NewProc("CoInitializeEx")
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

// RunLoginWindow opens a dedicated WebView2 sign-in window for Google/YouTube authentication
// and Brand Account channel selection.
func RunLoginWindow() error {
	procCoInitializeEx.Call(0, 2) // ensure STA COM on current OS thread
	dataDir, err := config.AuthWebViewProfileDir()
	if err != nil {
		return err
	}
	_ = os.MkdirAll(dataDir, 0o755)

	var hInstance windows.Handle
	_ = windows.GetModuleHandleEx(0, nil, &hInstance)

	chromium := edge.NewChromium()
	chromium.DataPath = dataDir

	var (
		hwnd            uintptr
		doneOnce        sync.Once
		stateMu         sync.Mutex
		webviewPtr      *edge.ICoreWebView2
		checkingAuth    bool
		visitedSwitcher bool
		pendingAuth     *config.AuthState
	)

	setWindowTitle := func(text string) {
		if hwnd == 0 {
			return
		}
		ptr, err := windows.UTF16PtrFromString(text)
		if err == nil {
			procSetWindowTextW.Call(hwnd, uintptr(unsafe.Pointer(ptr)))
		}
	}

	commitFinish := func() {
		stateMu.Lock()
		finalAuth := pendingAuth
		stateMu.Unlock()
		if finalAuth == nil {
			return
		}
		doneOnce.Do(func() {
			_ = config.SaveAuthState(*finalAuth)
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
			Email       string `json:"email"`
			PageID      string `json:"page_id"`
			AuthUser    string `json:"auth_user"`
			PagePath    string `json:"page_path"`
		}
		if err := json.Unmarshal([]byte(raw), &msg); err != nil || !msg.LoggedIn {
			return
		}
		if isIntermediateAuthPath(msg.PagePath) {
			return
		}

		stateMu.Lock()
		wv := webviewPtr
		if wv == nil || checkingAuth || pendingAuth != nil {
			stateMu.Unlock()
			return
		}
		checkingAuth = true
		stateMu.Unlock()

		extractAllCookies(wv, "https://www.youtube.com", func(cookies map[string]string, entries []config.CookieEntry) {
			if !hasRequiredSessionCookies(cookies) {
				stateMu.Lock()
				checkingAuth = false
				stateMu.Unlock()
				return
			}

			go func(baseCookies map[string]string, baseEntries []config.CookieEntry) {
				authState := config.AuthState{
					LoggedIn:      true,
					AccountName:   strings.TrimSpace(msg.AccountName),
					Email:         strings.TrimSpace(msg.Email),
					ActivePageID:  strings.TrimSpace(msg.PageID),
					AuthUser:      strings.TrimSpace(msg.AuthUser),
					Cookies:       baseCookies,
					CookieEntries: baseEntries,
					UpdatedAt:     time.Now().Unix(),
				}
				if authState.AuthUser == "" {
					authState.AuthUser = "0"
				}

				ctx, cancel := context.WithTimeout(context.Background(), 6*time.Second)
				defer cancel()
				ytClient := youtube.NewClient(nil)
				ytClient.SetAuth(authState)
				prof, profErr := ytClient.FetchAccountProfile(ctx)
				if profErr == nil {
					ApplyProfileToAuthState(&authState, prof)
				}

				stateMu.Lock()
				shouldOpenSwitcher := !visitedSwitcher && len(authState.Accounts) > 1
				if shouldOpenSwitcher {
					visitedSwitcher = true
					checkingAuth = false
					stateMu.Unlock()
					if hwnd != 0 {
						procPostMessageW.Call(hwnd, wmSwitchChannel, 0, 0)
					}
					return
				}

				if authState.AccountName == "" {
					if authState.Email != "" {
						authState.AccountName = authState.Email
					} else {
						authState.AccountName = "YouTube Account"
					}
				}
				pendingAuth = &authState
				checkingAuth = false
				stateMu.Unlock()

				if hwnd != 0 {
					procPostMessageW.Call(hwnd, wmFinishLogin, 0, 0)
				}
			}(cookies, entries)
		})
	}

	chromium.NavigationCompletedCallback = func(sender *edge.ICoreWebView2, _ *edge.ICoreWebView2NavigationCompletedEventArgs) {
		stateMu.Lock()
		webviewPtr = sender
		stateMu.Unlock()

		src := getWebViewSource(sender)
		u, err := url.Parse(src)
		if err != nil || !isYouTubeAppHost(u.Hostname()) {
			return
		}
		if strings.HasPrefix(u.Path, "/channel_switcher") {
			stateMu.Lock()
			visitedSwitcher = true
			stateMu.Unlock()
			setWindowTitle("Select YouTube Brand Account — ytmusic")
			return
		}
		if isIntermediateAuthPath(u.Path) {
			return
		}

		setWindowTitle("Signing in to YouTube — ytmusic")
		chromium.Eval(ytAuthProbeScript)
	}

	wndProc := windows.NewCallback(func(h, msg, wp, lp uintptr) uintptr {
		switch msg {
		case 0x0005: // WM_SIZE
			chromium.Resize()
			return 0
		case 0x0046, 0x0003: // WM_MOVE
			_ = chromium.NotifyParentWindowPositionChanged()
		case wmSwitchChannel:
			setWindowTitle("Select YouTube Brand Account — ytmusic")
			chromium.Navigate(youtubeSwitcherURL)
			return 0
		case wmFinishLogin:
			commitFinish()
			return 0
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
	winW, winH := uintptr(500), uintptr(680)
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

	// Catch SPA navigations off /channel_switcher or deferred ytcfg hydration cleanly.
	chromium.Init(`(function(){
		var h = (location.hostname || '').toLowerCase();
		if (h !== 'www.youtube.com' && h !== 'm.youtube.com' && h !== 'music.youtube.com' && h !== 'youtube.com') return;
		var timer = setInterval(function() {
			var p = location.pathname || '/';
			if (p.indexOf('/signin') === 0 || p.indexOf('/channel_switcher') === 0 || p.indexOf('/logout') === 0 || p.indexOf('/oops') === 0) {
				return;
			}
			var cfg = (window.ytcfg && window.ytcfg.data_) || null;
			if (!cfg || !cfg.LOGGED_IN) return;
			clearInterval(timer);
			` + ytAuthExtractBody + `
		}, 450);
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

const ytAuthExtractBody = `
	try {
		var cfg = (window.ytcfg && window.ytcfg.data_) || {};
		if (!cfg.LOGGED_IN) return;
		var name = cfg.USER_DISPLAY_NAME || '';
		if (!name) {
			var el = document.querySelector('#account-name, yt-formatted-string#account-name');
			if (el && el.textContent) name = el.textContent.trim();
		}
		var pageId = cfg.DELEGATED_SESSION_ID || '';
		if (!pageId && typeof cfg.DATASYNC_ID === 'string' && cfg.DATASYNC_ID.indexOf('||') > 0) {
			pageId = cfg.DATASYNC_ID.split('||')[0] || '';
		}
		var authUser = (cfg.SESSION_INDEX !== undefined && cfg.SESSION_INDEX !== null) ? String(cfg.SESSION_INDEX) : '0';
		window.chrome.webview.postMessage(JSON.stringify({
			type: 'auth_check',
			logged_in: true,
			account_name: name,
			email: cfg.ACCOUNT_EMAIL || '',
			page_id: pageId,
			auth_user: authUser,
			page_path: location.pathname || '/'
		}));
	} catch (e) {}
`

const ytAuthProbeScript = `(function(){
	var attempts = 0;
	function check() {
		attempts++;
		var p = location.pathname || '/';
		if (p.indexOf('/signin') === 0 || p.indexOf('/channel_switcher') === 0 || p.indexOf('/logout') === 0) return;
		var cfg = (window.ytcfg && window.ytcfg.data_) || null;
		if (cfg && cfg.LOGGED_IN) {
			` + ytAuthExtractBody + `
			return;
		}
		if (attempts < 12) {
			setTimeout(check, 350);
		}
	}
	check();
})();`

func isYouTubeAppHost(host string) bool {
	switch strings.ToLower(strings.TrimSpace(host)) {
	case "www.youtube.com", "m.youtube.com", "music.youtube.com", "youtube.com":
		return true
	default:
		return false
	}
}

func isIntermediateAuthPath(p string) bool {
	return strings.HasPrefix(p, "/signin") ||
		strings.HasPrefix(p, "/channel_switcher") ||
		strings.HasPrefix(p, "/logout") ||
		strings.HasPrefix(p, "/oops") ||
		strings.HasPrefix(p, "/select_site")
}

func hasRequiredSessionCookies(cookies map[string]string) bool {
	if len(cookies) == 0 {
		return false
	}
	hasAPISID := cookies["SAPISID"] != "" || cookies["__Secure-3PAPISID"] != ""
	hasSID := cookies["SID"] != "" || cookies["__Secure-3PSID"] != "" || cookies["__Secure-1PSID"] != ""
	return hasAPISID && hasSID
}

// ApplyProfileToAuthState merges InnerTube account metadata and Brand Accounts onto an AuthState.
func ApplyProfileToAuthState(authState *config.AuthState, prof youtube.AccountProfile) {
	if prof.SessionExpired && prof.AccountName == "" && len(prof.Accounts) == 0 {
		authState.NeedsReauth = true
		return
	}
	if prof.AccountName != "" {
		authState.AccountName = prof.AccountName
	}
	if prof.Email != "" {
		authState.Email = prof.Email
	}
	if len(prof.Accounts) > 0 {
		authState.Accounts = prof.Accounts
		authState.ChannelHandle = prof.ChannelHandle
		if prof.AvatarURL != "" {
			authState.AvatarURL = prof.AvatarURL
		}
		authState.ActivePageID = prof.ActivePageID
	} else {
		if prof.ChannelHandle != "" {
			authState.ChannelHandle = prof.ChannelHandle
		}
		if prof.AvatarURL != "" {
			authState.AvatarURL = prof.AvatarURL
		}
		if prof.ActivePageID != "" {
			authState.ActivePageID = prof.ActivePageID
		}
	}
	if prof.AuthUser != "" {
		authState.AuthUser = prof.AuthUser
	}
	authState.NeedsReauth = prof.SessionExpired
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

func extractAllCookies(wv *edge.ICoreWebView2, targetURI string, done func(cookies map[string]string, entries []config.CookieEntry)) {
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
		defer syscall.SyscallN(comVtblSlot(list, 2), uintptr(list)) // Release CookieList
		var count uint32
		// ICoreWebView2CookieList: slot 3 = get_Count
		syscall.SyscallN(comVtblSlot(list, 3), uintptr(list), uintptr(unsafe.Pointer(&count)))
		cookies := make(map[string]string, count)
		cookiePath := make(map[string]string, count)
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
				// Prefer root "/" path cookies when duplicate names exist across paths.
				if _, exists := cookies[name]; !exists || cpath == "/" || len(cpath) < len(cookiePath[name]) {
					cookies[name] = value
					cookiePath[name] = cpath
				}
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

	uri, _ := windows.UTF16PtrFromString(targetURI)
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
