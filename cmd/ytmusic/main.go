//go:build windows

package main

import (
	"fmt"
	"log"
	"os"
	"runtime"
	"strings"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"

	"ytmusic/internal/app"
	"ytmusic/internal/auth"
	"ytmusic/internal/config"
)

type cliOptions struct {
	debug      bool
	login      bool
	smokeTest  bool
	resetStats bool
	showHelp   bool
	unknownArg string
}

func parseCLIArgs(args []string) cliOptions {
	var opts cliOptions
	for _, arg := range args {
		switch strings.TrimSpace(arg) {
		case "--login":
			opts.login = true
		case "--debug":
			opts.debug = true
		case "--smoke-test":
			opts.smokeTest = true
		case "--reset-stats", "--clear-stats":
			opts.resetStats = true
		case "--help", "-h", "/?", "-help":
			opts.showHelp = true
		default:
			if strings.HasPrefix(arg, "-") && opts.unknownArg == "" {
				opts.unknownArg = arg
			}
		}
	}
	return opts
}

// FormatCLIHelp returns formatted usage documentation for all supported CLI options.
func FormatCLIHelp() string {
	return strings.TrimSpace(`ytmusic — Minimalistic Windows 11 YouTube Music & Video Player

Usage:
  ytmusic.exe [options]

Options:
  (no options)        Launch ytmusic (or restore an already-running instance)
  --debug             Enable WebView2 DevTools and debug logging
  --login             Open dedicated Google / YouTube Brand Account login window
  --reset-stats       Reset stored session & all-time listening time stats (alias: --clear-stats)
  --smoke-test        Run startup state, UI asset, and local proxy self-test and exit
  -h, --help          Show this command-line help message and exit`)
}

func main() {
	runtime.LockOSThread()

	opts := parseCLIArgs(os.Args[1:])

	if opts.unknownArg != "" {
		msg := fmt.Sprintf("Unknown option: %s\n\n%s", opts.unknownArg, FormatCLIHelp())
		writeCLIOutput("ytmusic — Invalid Option", msg)
		os.Exit(2)
	}

	if opts.showHelp {
		writeCLIOutput("ytmusic — Command-Line Options", FormatCLIHelp())
		return
	}

	if opts.resetStats {
		if err := config.ResetPlayTimeStats(); err != nil {
			writeCLIOutput("ytmusic — Reset Stats Error", fmt.Sprintf("Failed to reset listening time stats: %v", err))
			os.Exit(1)
		}
		notifyExistingInstanceResetStats()
		writeCLIOutput("ytmusic — Listening Stats Cleared", "Session and all-time listening time statistics have been reset to 0:00.")
		return
	}

	if opts.login {
		if err := auth.RunLoginWindow(); err != nil {
			log.Fatalf("login failed: %v", err)
		}
		return
	}

	if opts.smokeTest {
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
		showMessageBox("ytmusic startup error", err.Error(), 0x10 /* MB_ICONERROR */)
		os.Exit(1)
	}

	if err := application.Run(opts.debug); err != nil {
		showMessageBox("ytmusic runtime error", err.Error(), 0x10 /* MB_ICONERROR */)
		os.Exit(1)
	}
}

func focusExistingInstance() {
	postMessageToRunningInstance(0x800B /* wmRestoreApp */)
}

func notifyExistingInstanceResetStats() {
	postMessageToRunningInstance(0x800C /* wmResetStats */)
}

func postMessageToRunningInstance(msg uint32) {
	user32 := windows.NewLazySystemDLL("user32.dll")
	procFindWindowW := user32.NewProc("FindWindowW")
	procPostMessageW := user32.NewProc("PostMessageW")

	className, _ := windows.UTF16PtrFromString("webview")
	title, _ := windows.UTF16PtrFromString("ytmusic")
	hwnd, _, _ := procFindWindowW.Call(uintptr(unsafe.Pointer(className)), uintptr(unsafe.Pointer(title)))
	if hwnd != 0 {
		procPostMessageW.Call(hwnd, uintptr(msg), 0, 0)
	}
}

func writeCLIOutput(title, text string) {
	// If stdout is redirected/piped (e.g. pwsh capture or > file), write directly to stdout.
	if h, err := windows.GetStdHandle(windows.STD_OUTPUT_HANDLE); err == nil && h != 0 && h != windows.InvalidHandle {
		if ft, err := windows.GetFileType(h); err == nil && (ft == windows.FILE_TYPE_DISK || ft == windows.FILE_TYPE_PIPE) {
			fmt.Println(text)
			return
		}
	}

	// For interactive terminal runs of a -H=windowsgui binary, attach to the parent console.
	kernel32 := windows.NewLazySystemDLL("kernel32.dll")
	procAttachConsole := kernel32.NewProc("AttachConsole")
	const attachParentProcess = ^uintptr(0)
	if attached, _, _ := procAttachConsole.Call(attachParentProcess); attached != 0 {
		if hOut, err := windows.GetStdHandle(windows.STD_OUTPUT_HANDLE); err == nil && hOut != 0 && hOut != windows.InvalidHandle {
			msgUTF16, _ := windows.UTF16FromString("\n" + text + "\n")
			if len(msgUTF16) > 1 {
				var written uint32
				_ = windows.WriteConsole(hOut, &msgUTF16[0], uint32(len(msgUTF16)-1), &written, nil)
				return
			}
		}
	}

	// Launched directly via Explorer/Run dialog with no console: display native dialog.
	showMessageBox(title, text, 0x40 /* MB_ICONINFORMATION */)
}

func showMessageBox(title, message string, flags uintptr) {
	user32 := windows.NewLazySystemDLL("user32.dll")
	procMessageBoxW := user32.NewProc("MessageBoxW")
	t, _ := windows.UTF16PtrFromString(title)
	m, _ := windows.UTF16PtrFromString(message)
	procMessageBoxW.Call(0, uintptr(unsafe.Pointer(m)), uintptr(unsafe.Pointer(t)), flags)
}
