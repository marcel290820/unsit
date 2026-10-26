package tray

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
	"syscall"
	"unsafe"

	"fyne.io/systray"

	"unsit/assets"
	"unsit/internal/timer"
)

var (
	kernel32 = syscall.NewLazyDLL("kernel32.dll")
	user32   = syscall.NewLazyDLL("user32.dll")
	// The thread systray runs its message loop on: main's, which systray
	// locks to the main OS thread at init.
	loopThread uintptr
)

// setup checks for a taskbar to put the icon in, so a desktop without one
// falls back at once instead of after abort's wait.
func setup(context.Context, timer.State) error {
	loopThread, _, _ = kernel32.NewProc("GetCurrentThreadId").Call()
	findWindow := user32.NewProc("FindWindowW")
	if taskbar, _, _ := findWindow.Call(uintptr(unsafe.Pointer(syscall.StringToUTF16Ptr("Shell_TrayWnd"))), 0); taskbar == 0 {
		return errors.New("no taskbar to show a tray icon in")
	}
	return nil
}

// systray calls back only once the icon is added.
func registered(context.Context) bool { return true }

// started closes the console window Windows opens for unsit when it starts
// from Explorer or a Startup-folder shortcut: the tray icon is the whole UI.
// A console shared with a shell (unsit typed into a terminal) stays.
func started() {
	var pids [2]uint32
	n, _, _ := kernel32.NewProc("GetConsoleProcessList").Call(uintptr(unsafe.Pointer(&pids[0])), uintptr(len(pids)))
	if n == 1 {
		kernel32.NewProc("FreeConsole").Call()
	}
}

// abort ends systray's message loop after a failed start, which left no
// window to close. (setup's FindWindowW gave the thread its message queue.)
func abort() {
	const wmQuit = 0x0012
	user32.NewProc("PostThreadMessageW").Call(loopThread, wmQuit, 0, 0)
}

func iconFor(p timer.Phase) []byte {
	light := lightTaskbar()
	switch {
	case p == timer.Standing && light:
		return assets.StandBlackICO
	case p == timer.Standing:
		return assets.StandWhiteICO
	case light:
		return assets.SitBlackICO
	default:
		return assets.SitWhiteICO
	}
}

func setIcon(icon []byte) {
	systray.SetIcon(icon)
	// systray hands the icon to Windows through a file it leaves in %TEMP%.
	// Windows has loaded it by now and systray caches the handle, so remove
	// the file to leave nothing behind. Best effort; a leftover is harmless.
	sum := md5.Sum(icon)
	_ = os.Remove(filepath.Join(os.TempDir(), "systray_temp_icon_"+hex.EncodeToString(sum[:])))
}

// lightTaskbar reports whether the taskbar uses the light theme (Settings >
// Personalization > Colors > "Choose your default Windows mode").
func lightTaskbar() bool {
	var key syscall.Handle
	path := syscall.StringToUTF16Ptr(`Software\Microsoft\Windows\CurrentVersion\Themes\Personalize`)
	if syscall.RegOpenKeyEx(syscall.HKEY_CURRENT_USER, path, 0, syscall.KEY_READ, &key) != nil {
		return false // no setting: Windows' default, dark
	}
	defer syscall.RegCloseKey(key)
	var light, size uint32 = 0, 4
	err := syscall.RegQueryValueEx(key, syscall.StringToUTF16Ptr("SystemUsesLightTheme"), nil, nil, (*byte)(unsafe.Pointer(&light)), &size)
	return err == nil && light == 1
}
