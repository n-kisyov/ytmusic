//go:build windows

package auth

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"runtime"
	"testing"
	"time"
	"unsafe"

	"github.com/jchv/go-webview2/pkg/edge"
	"golang.org/x/sys/windows"

	"ytmusic/internal/config"
)

func TestExtractAllCookiesCapturesHttpOnly(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping live WebView2 CookieManager test in -short mode")
	}

	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	ole32 := windows.NewLazySystemDLL("ole32.dll")
	procCoInit := ole32.NewProc("CoInitializeEx")
	procCoUninit := ole32.NewProc("CoUninitialize")
	hrCo, _, _ := procCoInit.Call(0, 2) // COINIT_APARTMENTTHREADED
	if hrCo == 0 || hrCo == 1 {
		defer procCoUninit.Call()
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.SetCookie(w, &http.Cookie{Name: "regular_ck", Value: "val1", Path: "/"})
		http.SetCookie(w, &http.Cookie{Name: "http_only_ck", Value: "secret_xyz", Path: "/", HttpOnly: true})
		w.Header().Set("Content-Type", "text/html")
		_, _ = fmt.Fprint(w, "<html><body>ok</body></html>")
	}))
	defer srv.Close()

	var hInstance windows.Handle
	_ = windows.GetModuleHandleEx(0, nil, &hInstance)

	chromium := edge.NewChromium()
	chromium.DataPath = t.TempDir()

	var hwnd uintptr
	wndProc := windows.NewCallback(func(h, msg, wp, lp uintptr) uintptr {
		if msg == 0x0002 {
			procPostQuitMessage.Call(0)
			return 0
		}
		r, _, _ := procDefWindowProcW.Call(h, msg, wp, lp)
		return r
	})

	className, _ := windows.UTF16PtrFromString("YTMusicCookieTestWin")
	wc := wndClassExW{
		CbSize:        uint32(unsafe.Sizeof(wndClassExW{})),
		HInstance:     hInstance,
		LpszClassName: className,
		LpfnWndProc:   wndProc,
	}
	procRegisterClassExW.Call(uintptr(unsafe.Pointer(&wc)))

	hwnd, _, _ = procCreateWindowExW.Call(
		0, uintptr(unsafe.Pointer(className)), uintptr(unsafe.Pointer(className)),
		0x00CF0000, 0, 0, 200, 200, 0, 0, uintptr(hInstance), 0,
	)
	if hwnd == 0 {
		t.Fatal("CreateWindowExW failed")
	}

	var gotCookies map[string]string
	var gotEntries []config.CookieEntry
	done := false

	chromium.NavigationCompletedCallback = func(sender *edge.ICoreWebView2, _ *edge.ICoreWebView2NavigationCompletedEventArgs) {
		extractAllCookies(sender, srv.URL, func(cookies map[string]string, entries []config.CookieEntry) {
			gotCookies = cookies
			gotEntries = entries
			done = true
			procDestroyWindow.Call(hwnd)
		})
	}

	if !chromium.Embed(hwnd) {
		procDestroyWindow.Call(hwnd)
		t.Fatal("chromium.Embed failed")
	}
	chromium.Navigate(srv.URL + "/")

	deadline := time.Now().Add(8 * time.Second)
	var m msgStruct
	for !done && time.Now().Before(deadline) {
		ret, _, _ := procGetMessageW.Call(uintptr(unsafe.Pointer(&m)), 0, 0, 0)
		if int32(ret) <= 0 {
			break
		}
		procTranslateMessage.Call(uintptr(unsafe.Pointer(&m)))
		procDispatchMessageW.Call(uintptr(unsafe.Pointer(&m)))
	}

	if !done {
		procDestroyWindow.Call(hwnd)
		t.Fatal("timed out waiting for WebView2 cookie extraction")
	}
	if gotCookies["regular_ck"] != "val1" {
		t.Fatalf("expected regular_ck=val1, got %q", gotCookies["regular_ck"])
	}
	if gotCookies["http_only_ck"] != "secret_xyz" {
		t.Fatalf("expected HttpOnly cookie http_only_ck=secret_xyz, got %q (cookies=%v)", gotCookies["http_only_ck"], gotCookies)
	}
	foundHTTPOnlyFlag := false
	for _, e := range gotEntries {
		if e.Name == "http_only_ck" && e.HTTPOnly {
			foundHTTPOnlyFlag = true
			break
		}
	}
	if !foundHTTPOnlyFlag {
		t.Fatalf("expected CookieEntry http_only_ck with HTTPOnly=true, got %+v", gotEntries)
	}
}
