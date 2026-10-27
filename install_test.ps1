# Runs install.ps1 against a fake release of the unsit.exe built in CI, then
# uninstall.ps1, and checks that the uninstall leaves nothing behind. CI only:
# it installs and removes unsit for the current user.

$ErrorActionPreference = 'Stop'
if (-not $env:CI) { throw 'install_test.ps1 runs only in CI: it uninstalls unsit' }
# Brackets, as a user name may have, must not be read as wildcards.
$env:LOCALAPPDATA = (New-Item -ItemType Directory -Force (Join-Path $env:RUNNER_TEMP 'local [app] data')).FullName
$env:TMP = $env:TEMP = (New-Item -ItemType Directory -Force (Join-Path $env:RUNNER_TEMP 'temp [1]')).FullName
$dir = Join-Path $env:LOCALAPPDATA 'Programs\unsit'
$startup = Join-Path ([Environment]::GetFolderPath('Startup')) 'unsit.lnk'
$aumid = 'Software\Classes\AppUserModelId\unsit'
function Check($what, $ok) { if (-not $ok) { throw "FAIL $what" }; Write-Host "ok   $what" }
function UserPath { [Environment]::GetEnvironmentVariable('Path', 'User') -split ';' }

$release = Join-Path $env:RUNNER_TEMP 'release'
New-Item -ItemType Directory -Force $release | Out-Null
Compress-Archive -Path unsit.exe, assets\unsit.ico -DestinationPath "$release\unsit_windows_amd64.zip"
$sum = (Get-FileHash -Algorithm SHA256 "$release\unsit_windows_amd64.zip").Hash.ToLower()
Set-Content "$release\checksums.txt" "$sum  unsit_windows_amd64.zip"
$server = Start-Process python -ArgumentList '-m', 'http.server', '8765', '--bind', '127.0.0.1', '--directory', $release -PassThru -WindowStyle Hidden
try {
    $installer = (Get-Content -Raw install.ps1) -replace '\$url = "[^"]*"', '$url = "http://127.0.0.1:8765"'
    for ($i = 0; ; $i++) {
        try { Invoke-WebRequest -UseBasicParsing http://127.0.0.1:8765/checksums.txt | Out-Null; break }
        catch { if ($i -ge 50) { throw } ; Start-Sleep -Milliseconds 200 }
    }
    Invoke-Expression $installer
} finally {
    $server | Stop-Process
}
Check 'install puts unsit.exe and unsit.ico in place' ((Test-Path -LiteralPath "$dir\unsit.exe") -and (Test-Path -LiteralPath "$dir\unsit.ico"))
Check 'install adds the folder to the user PATH' ((UserPath) -contains $dir)

# What a user may have added since: the README's Startup shortcut, and the
# notification registration unsit makes.
$shortcut = (New-Object -ComObject WScript.Shell).CreateShortcut($startup)
$shortcut.TargetPath = "$dir\unsit.exe"
$shortcut.Save()
[Microsoft.Win32.Registry]::CurrentUser.CreateSubKey($aumid).Close()

Invoke-Expression (Get-Content -Raw uninstall.ps1)
Check 'uninstall removes the folder' (-not (Test-Path -LiteralPath $dir))
Check 'uninstall removes the PATH entry' ((UserPath) -notcontains $dir)
Check 'uninstall removes the Startup shortcut' (-not (Test-Path $startup))
Check 'uninstall removes the notification registration' ($null -eq [Microsoft.Win32.Registry]::CurrentUser.OpenSubKey($aumid))
