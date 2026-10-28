<h1>
<picture>
  <source media="(prefers-color-scheme: dark)" srcset="assets/stand-white.png">
  <img src="assets/stand-black.png" width="40" alt="">
</picture>
unsit
</h1>

A tiny sit/stand timer. Run `unsit` and it counts 40 minutes of sitting, then
20 of standing, round and round, with a quiet notification at each switch.
Stop it from the tray icon or with Ctrl-C. That is the whole program.

Runs on macOS 13+, Windows 10/11 and Linux desktops with a system tray, made
with Omarchy in mind (Hyprland with Waybar or Omarchy's own bar).

## What it does

- Starts sitting the moment you run it.
- After 40 minutes, a notification: **Stand up**, *20 minutes standing*.
  After 20 more: **Sit down**, *40 minutes sitting*. Then again, until you stop it.
- Shows a tray icon of a seated or standing figure. Its menu shows the time
  left (`Sitting · 23 min left`) and **Stop**. The macOS menu bar also shows
  `Sit 23m` next to the icon; on Windows and Linux, hover the icon for the time.
- Survives sleep. Phases end at wall-clock times, so a laptop closed through a
  switch gets one notification when it wakes, and the next phase starts then.
  You never get a backlog.
- Remembers nothing. Every run starts fresh.

## What it deliberately doesn't do

No forced breaks, screen overlays, input locking or nagging. No autostart (you
can set that up yourself, below). No statistics, streaks or history. No idle,
calendar, camera or meeting detection. No accounts, network access, telemetry
or updater. No settings window and no config file: the flags below are all the
settings there are.

## Install

| Platform        | Command                                |
|-----------------|----------------------------------------|
| macOS           | `brew install marcel290820/tap/unsit`  |
| Windows 10/11   | `winget install unsit`                 |

Each puts `unsit` on your PATH; open a new terminal and run it. On Arch and
Omarchy, use `install.sh` below for now: an AUR package is planned.

No package manager? These download the latest release, check it against the
published checksum and install it without admin rights:

```sh
# macOS, Linux: installs to ~/.local/bin
curl -fsSL https://raw.githubusercontent.com/marcel290820/unsit/main/install.sh | sh
```

```powershell
# Windows: installs to %LOCALAPPDATA%\Programs\unsit and adds it to your PATH
irm https://raw.githubusercontent.com/marcel290820/unsit/main/install.ps1 | iex
```

To remove unsit again, see [Uninstall](#uninstall).

## Usage

```sh
unsit                          # 40 minutes sitting, 20 standing
unsit --sit 50m --stand 10m    # your own rhythm
unsit --no-tray                # no tray icon; prints a line at each switch
```

| Flag          | Default | Meaning                                              |
|---------------|---------|------------------------------------------------------|
| `--sit`       | `40m`   | How long to sit. Go durations: `45m`, `1h`, `1h30m`, `10s`. |
| `--stand`     | `20m`   | How long to stand.                                   |
| `--no-tray`   |         | Skip the tray icon. Notifications still come, and each switch prints a line. |
| `--version`   |         | Print the version.                                   |
| `--help`      |         | Print help.                                          |

To stop, pick **Stop** in the tray menu, or press Ctrl-C (SIGTERM works too).
unsit exits at once and sends no notification on the way out.

If unsit can't show a tray icon (on Linux, when nothing on the desktop hosts
StatusNotifierItem icons), it waits up to 5 seconds for a tray to appear, then
carries on as if you had passed `--no-tray`, with a one-line warning.

### Try it

```sh
unsit --sit 10s --stand 5s
```

1. The seated icon appears in the tray. Its menu reads `Sitting · 1 min left`.
2. After 10 seconds: a **Stand up** notification, and the icon stands up.
3. After 5 more: **Sit down**, and it sits again. It keeps going.
4. **Stop** in the menu ends it. Run it again and try Ctrl-C.

`unsit --no-tray --sit 10s --stand 5s` does the same without the icon and
prints each switch.

## Platform notes

**Linux.** The icon is a StatusNotifierItem, which Omarchy's bar, Waybar's
`tray` module and KDE show; GNOME needs the AppIndicator extension.
Notifications go to whatever handles freedesktop notifications (mako,
Omarchy's shell, dunst, GNOME, KDE), at normal urgency with your daemon's
default timeout; each one replaces the last. The icon is white, or black when
your desktop prefers a light theme (read from the XDG settings portal, which
Omarchy updates when you switch themes). Omarchy keeps tray icons in a drawer:
hover the tray's arrow in the bar to see unsit. On Omarchy 4, right-click that
arrow to pin unsit so it stays in view.

**macOS.** The menu bar icon is a template image, so it matches light and dark
menu bars. Notifications come through AppleScript, so macOS files them under
*Script Editor*. If none appear, allow notifications for Script Editor in
System Settings > Notifications.

**Windows.** The icon is white on a dark taskbar and black on a light one.
Notifications come from *unsit*, with its orange icon; to silence them or
change their sound, look for unsit under Settings > System > Notifications.
Started from Explorer or a Startup shortcut, unsit closes the console window
Windows opens for it; started from a terminal, it stays there and Ctrl-C
works.

## Start at login (optional)

unsit never sets this up itself. If you want it, pick your platform.

**macOS.** A LaunchAgent starts unsit at every login, without a Terminal
window (a Login Item would open one):

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

It takes effect at your next login. To undo it, delete that file.

**Windows.** Put a shortcut in your Startup folder. In PowerShell:

```powershell
$s = (New-Object -ComObject WScript.Shell).CreateShortcut("$([Environment]::GetFolderPath('Startup'))\unsit.lnk")
$s.TargetPath = (Get-Command unsit).Source
$s.WindowStyle = 7  # start minimized
$s.Save()
```

To undo it, delete `unsit.lnk` from the folder `shell:startup` opens.

**Hyprland and Omarchy.** Add one line to your Hyprland config:

| Setup                                   | File                           | Line                              |
|-----------------------------------------|--------------------------------|-----------------------------------|
| Omarchy 4                               | `~/.config/hypr/autostart.lua` | `o.launch_on_start("unsit")`      |
| Omarchy 3                               | `~/.config/hypr/autostart.conf`| `exec-once = uwsm-app -- unsit`   |
| Hyprland with a `hyprland.conf`         | `~/.config/hypr/hyprland.conf` | `exec-once = unsit`               |

unsit waits a few seconds for the bar's tray, so starting both together is fine.

## Uninstall

| Installed with  | Remove with                                                     |
|-----------------|-----------------------------------------------------------------|
| Homebrew        | `brew uninstall --zap unsit`                                    |
| winget          | `winget uninstall unsit`                                        |
| `install.sh`    | `rm ~/.local/bin/unsit`                                         |
| `install.ps1`   | `irm https://raw.githubusercontent.com/marcel290820/unsit/main/uninstall.ps1 \| iex` |

`--zap` also deletes the LaunchAgent from [Start at login](#start-at-login-optional),
and `uninstall.ps1` also deletes the Startup shortcut and the PATH entry. In
the other cases, undo autostart yourself: delete `~/Library/LaunchAgents/unsit.plist`
or `unsit.lnk` in `shell:startup`, or remove the line from your Hyprland config.

## Privacy

unsit makes no network connections and keeps no settings, history or logs.
On Windows it registers its name and icon for notifications under
`HKEY_CURRENT_USER\Software\Classes\AppUserModelId\unsit`, as a volatile key
that Windows drops at sign-out, and the tray library hands each tray icon to
Windows through a file in `%TEMP%`, which unsit deletes right away. So once
uninstalled, unsit leaves nothing behind after your next sign-out, apart from
any autostart entry you added yourself.

## Building

With Go 1.26 or newer:

```sh
go build ./cmd/unsit    # macOS needs the Xcode command line tools (cgo)
go test ./...           # Linux end-to-end tests need dbus-daemon
go generate ./assets    # redraws the icons
```

The code is small on purpose: `internal/timer` is the sit/stand cycle as a
pure state machine, `internal/tray` the icon and menu, `internal/notify` one
short file per OS, and `cmd/unsit` ties them together. Releases are built by
GoReleaser; see [RELEASING.md](RELEASING.md).

## License

[MIT](LICENSE)
