<h1>
<picture>
  <source media="(prefers-color-scheme: dark)" srcset="assets/stand-white.png">
  <img src="assets/stand-black.png" width="40" alt="">
</picture>
unsit
</h1>

A tiny sit/stand timer. Run `unsit` and it counts 40 minutes of sitting, then
20 of standing, round and round, with a notification at each switch. The tray
icon shows the current phase and the time left; its menu has **Skip** and
**Stop**. No settings, no history, no network.

Runs on macOS 13+, Windows 10/11 and Linux desktops with a system tray.

## Usage

```sh
unsit                          # 40 minutes sitting, 20 standing
unsit --sit 50m --stand 10m    # your own rhythm
unsit --no-tray                # no tray icon; prints a line at each switch
```

Stop it from the tray menu or with Ctrl-C. `unsit --help` lists all flags.

## Install

| Platform      | Command                                     |
|---------------|---------------------------------------------|
| macOS         | `brew install marcelheidebrecht/tap/unsit`  |
| Windows 10/11 | `winget install unsit`                      |

Linux, or no package manager:

```sh
# macOS, Linux: installs to ~/.local/bin
curl -fsSL https://raw.githubusercontent.com/marcelheidebrecht/unsit/main/install.sh | sh
```

```powershell
# Windows: installs to %LOCALAPPDATA%\Programs\unsit
irm https://raw.githubusercontent.com/marcelheidebrecht/unsit/main/install.ps1 | iex
```

### Uninstall

| Installed with | Remove with                                                                               |
|----------------|-------------------------------------------------------------------------------------------|
| Homebrew       | `brew uninstall --zap unsit`                                                              |
| winget         | `winget uninstall unsit`                                                                  |
| `install.sh`   | `rm ~/.local/bin/unsit`                                                                   |
| `install.ps1`  | `irm https://raw.githubusercontent.com/marcelheidebrecht/unsit/main/uninstall.ps1 \| iex` |

## Notes

- **Linux:** the tray icon needs a StatusNotifierItem host (Waybar, Omarchy, KDE;
  GNOME needs the AppIndicator extension). On Omarchy it sits in the tray drawer.
- **macOS:** notifications arrive as *Script Editor*. If none show up, allow
  them in System Settings > Notifications.

<details>
<summary><b>Start at login</b></summary>

**macOS** (undo: delete the file):

```sh
cat > ~/Library/LaunchAgents/unsit.plist <<EOF
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0"><dict>
  <key>Label</key><string>unsit</string>
  <key>ProgramArguments</key><array><string>$(command -v unsit)</string></array>
  <key>RunAtLoad</key><true/>
</dict></plist>
EOF
```

**Windows** (undo: delete `unsit.lnk` from `shell:startup`):

```powershell
$s = (New-Object -ComObject WScript.Shell).CreateShortcut("$([Environment]::GetFolderPath('Startup'))\unsit.lnk")
$s.TargetPath = (Get-Command unsit).Source
$s.WindowStyle = 7
$s.Save()
```

**Hyprland / Omarchy:** add `o.launch_on_start("unsit")` to
`~/.config/hypr/autostart.lua` (Omarchy 4), `exec-once = uwsm-app -- unsit` to
`autostart.conf` (Omarchy 3), or `exec-once = unsit` to `hyprland.conf`.

</details>

## Building

```sh
go build ./cmd/unsit    # Go 1.26+; macOS needs the Xcode command line tools
go test ./...
```

Releases: [RELEASING.md](RELEASING.md). License: [MIT](LICENSE).
