package notify

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/binary"
	"fmt"
	"html"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"unicode/utf16"
)

// Windows shows a toast under the name and icon registered for its app ID.
const appID = "unsit"

// The scripts take their input from the environment, which needs no quoting:
// UNSIT_APP_ID, UNSIT_ICON (a path, or empty) and UNSIT_TOAST (the toast XML).

// registerScript registers the app ID for the current user. The key is
// volatile: Windows drops it at sign-out, so an uninstalled unsit leaves none
// behind. Its shared parent is made lasting, as a volatile one would refuse
// other apps' lasting keys.
const registerScript = `
$ErrorActionPreference = 'Stop'
$apps = [Microsoft.Win32.Registry]::CurrentUser.CreateSubKey('Software\Classes\AppUserModelId')
$app = $apps.CreateSubKey($env:UNSIT_APP_ID, [Microsoft.Win32.RegistryKeyPermissionCheck]::ReadWriteSubTree, [Microsoft.Win32.RegistryOptions]::Volatile)
$app.SetValue('DisplayName', 'unsit')
if ($env:UNSIT_ICON) { $app.SetValue('IconUri', $env:UNSIT_ICON) } else { $app.DeleteValue('IconUri', $false) }
$app.Close()
$apps.Close()
`

// The same tag and group make each toast replace the previous one.
const showScript = `
[Windows.UI.Notifications.ToastNotificationManager, Windows.UI.Notifications, ContentType = WindowsRuntime] | Out-Null
[Windows.UI.Notifications.ToastNotification, Windows.UI.Notifications, ContentType = WindowsRuntime] | Out-Null
[Windows.Data.Xml.Dom.XmlDocument, Windows.Data.Xml.Dom.XmlDocument, ContentType = WindowsRuntime] | Out-Null
$xml = New-Object Windows.Data.Xml.Dom.XmlDocument
$xml.LoadXml($env:UNSIT_TOAST)
$toast = New-Object Windows.UI.Notifications.ToastNotification $xml
$toast.Tag = 'unsit'
$toast.Group = 'unsit'
[Windows.UI.Notifications.ToastNotificationManager]::CreateToastNotifier($env:UNSIT_APP_ID).Show($toast)
`

func send(ctx context.Context, title, body string) error {
	toast := fmt.Sprintf(`<toast><visual><binding template="ToastGeneric"><text>%s</text><text>%s</text></binding></visual></toast>`,
		html.EscapeString(title), html.EscapeString(body))
	return powershell(ctx, registerScript+showScript, "UNSIT_APP_ID="+appID, "UNSIT_ICON="+icon(), "UNSIT_TOAST="+toast)
}

// icon returns the path of unsit.ico, which releases ship next to unsit.exe,
// or "" if it isn't there (say, a go build): toasts then show no icon.
func icon() string {
	exe, err := os.Executable()
	if err == nil {
		exe, err = filepath.EvalSymlinks(exe) // winget runs unsit through a link
	}
	if err != nil {
		return ""
	}
	ico := filepath.Join(filepath.Dir(exe), "unsit.ico")
	if _, err := os.Stat(ico); err != nil {
		return ""
	}
	return ico
}

// powershell runs script with env added to unsit's environment.
func powershell(ctx context.Context, script string, env ...string) error {
	// -EncodedCommand takes base64 UTF-16LE: no quoting and no temp script file.
	var enc bytes.Buffer
	if err := binary.Write(&enc, binary.LittleEndian, utf16.Encode([]rune(script))); err != nil {
		return err
	}
	cmd := exec.CommandContext(ctx, "powershell.exe", "-NoProfile", "-NonInteractive",
		"-EncodedCommand", base64.StdEncoding.EncodeToString(enc.Bytes()))
	const createNoWindow = 0x08000000
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: createNoWindow}
	cmd.Env = append(os.Environ(), env...)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("toast via PowerShell: %w: %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}
