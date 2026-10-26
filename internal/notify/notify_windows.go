package notify

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/binary"
	"fmt"
	"html"
	"os/exec"
	"strings"
	"syscall"
	"unicode/utf16"
)

// Toasts must name a registered app. Registering our own would leave
// registry entries behind, so borrow Windows PowerShell's, which every
// Windows 10/11 install has; toasts show "Windows PowerShell" as the source.
const appID = `{1AC14E77-02E7-4E5D-B744-2EB1AE5198B7}\WindowsPowerShell\v1.0\powershell.exe`

// The same tag and group make each toast replace the previous one.
const script = `
$ErrorActionPreference = 'Stop'
[Windows.UI.Notifications.ToastNotificationManager, Windows.UI.Notifications, ContentType = WindowsRuntime] | Out-Null
[Windows.UI.Notifications.ToastNotification, Windows.UI.Notifications, ContentType = WindowsRuntime] | Out-Null
[Windows.Data.Xml.Dom.XmlDocument, Windows.Data.Xml.Dom.XmlDocument, ContentType = WindowsRuntime] | Out-Null
$xml = New-Object Windows.Data.Xml.Dom.XmlDocument
$xml.LoadXml('%s')
$toast = New-Object Windows.UI.Notifications.ToastNotification $xml
$toast.Tag = 'unsit'
$toast.Group = 'unsit'
[Windows.UI.Notifications.ToastNotificationManager]::CreateToastNotifier('%s').Show($toast)
`

func send(ctx context.Context, title, body string) error {
	toast := fmt.Sprintf(`<toast><visual><binding template="ToastGeneric"><text>%s</text><text>%s</text></binding></visual></toast>`,
		html.EscapeString(title), html.EscapeString(body))
	ps := fmt.Sprintf(script, strings.ReplaceAll(toast, "'", "''"), appID)

	// -EncodedCommand takes base64 UTF-16LE: no quoting and no temp script file.
	var enc bytes.Buffer
	if err := binary.Write(&enc, binary.LittleEndian, utf16.Encode([]rune(ps))); err != nil {
		return err
	}
	cmd := exec.CommandContext(ctx, "powershell.exe", "-NoProfile", "-NonInteractive",
		"-EncodedCommand", base64.StdEncoding.EncodeToString(enc.Bytes()))
	const createNoWindow = 0x08000000
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: createNoWindow}
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("toast via PowerShell: %w: %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}
