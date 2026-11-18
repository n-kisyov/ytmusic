//go:build windows

package main

import (
	"log"
	"os"
	"runtime"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"

	"ytmusic/internal/app"
	"ytmusic/internal/auth"
)

func main() {
	runtime.LockOSThread()

	debugMode := false
	isLoginMode := false
	isSmokeTest := false

	for _, arg := range os.Args[1:] {
		switch arg {
		case "--login":
			isLoginMode = true
		case "--debug":
			debugMode = true
		case "--smoke-test":
			isSmokeTest = true
		}
	}

	if isLoginMode {
		if err := auth.RunLoginWindow(); err != nil {
			log.Fatalf("login failed: %v", err)
		}
		return
	}

	if isSmokeTest {
		application, err := app.New()
		if err != nil {
			log.Fatalf("smoke test init failed: %v", err)
		}
		if err := application.RunSmokeTest(); err != nil {
			log.Fatalf("smoke test failed: %v", err)
		}
		return
	}

	// Ensure a single main instance; if already running, bring the existing window to front.
	mutexName, _ := windows.UTF16PtrFromString("Local\\YTMusicSingleInstanceMutex")
	mutex, err := windows.CreateMutex(nil, false, mutexName)
	if err == syscall.ERROR_ALREADY_EXISTS {
		if mutex != 0 {
			_ = windows.CloseHandle(mutex)
		}
		focusExistingInstance()
		return
	}
	if mutex != 0 {
		defer windows.CloseHandle(mutex)
	}

	application, err := app.New()
	if err != nil {
		showErrorMessageBox("ytmusic startup error", err.Error())
		os.Exit(1)
	}

	if err := application.Run(debugMode); err != nil {
		showErrorMessageBox("ytmusic runtime error", err.Error())
		os.Exit(1)
	}
}

func focusExistingInstance() {
	user32 := windows.NewLazySystemDLL("user32.dll")
	procFindWindowW := user32.NewProc("FindWindowW")
	procPostMessageW := user32.NewProc("PostMessageW")

	className, _ := windows.UTF16PtrFromString("webview")
	title, _ := windows.UTF16PtrFromString("ytmusic")
	hwnd, _, _ := procFindWindowW.Call(uintptr(unsafe.Pointer(className)), uintptr(unsafe.Pointer(title)))
	if hwnd != 0 {
		procPostMessageW.Call(hwnd, 0x800B /* wmRestoreApp */, 0, 0)
	}
}

func showErrorMessageBox(title, message string) {
	user32 := windows.NewLazySystemDLL("user32.dll")
	procMessageBoxW := user32.NewProc("MessageBoxW")
	t, _ := windows.UTF16PtrFromString(title)
	m, _ := windows.UTF16PtrFromString(message)
	procMessageBoxW.Call(0, uintptr(unsafe.Pointer(m)), uintptr(unsafe.Pointer(t)), 0x10 /* MB_ICONERROR */)
}
