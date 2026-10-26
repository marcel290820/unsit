# Installs the latest unsit release for Windows into
# %LOCALAPPDATA%\Programs\unsit and adds that folder to your user PATH.
# No admin rights; writes nothing else.
#
#   irm https://raw.githubusercontent.com/marcel290820/unsit/main/install.ps1 | iex
#
# Uninstall: delete %LOCALAPPDATA%\Programs\unsit and remove it from your user
# PATH (Settings > System > About > Advanced system settings > Environment Variables).

& {
    $ErrorActionPreference = 'Stop'
    $ProgressPreference = 'SilentlyContinue' # the progress bar slows downloads a lot in Windows PowerShell
    [Net.ServicePointManager]::SecurityProtocol = [Net.SecurityProtocolType]::Tls12

    $repo = 'marcel290820/unsit'
    $dir = Join-Path $env:LOCALAPPDATA 'Programs\unsit'
    $archive = 'unsit_windows_amd64.zip' # also runs on Windows on Arm, emulated
    $url = "https://github.com/$repo/releases/latest/download"

    $tmp = Join-Path ([IO.Path]::GetTempPath()) ("unsit-" + [Guid]::NewGuid())
    New-Item -ItemType Directory -Path $tmp | Out-Null
    try {
        Write-Host "Downloading $archive"
        Invoke-WebRequest -UseBasicParsing -Uri "$url/$archive" -OutFile "$tmp\$archive"
        Invoke-WebRequest -UseBasicParsing -Uri "$url/checksums.txt" -OutFile "$tmp\checksums.txt"

        $want = foreach ($line in Get-Content "$tmp\checksums.txt") {
            if ($line -match "^([0-9a-f]{64})\s+$([regex]::Escape($archive))$") { $Matches[1] }
        }
        $got = (Get-FileHash -Algorithm SHA256 "$tmp\$archive").Hash.ToLower()
        if (-not $want -or $want -ne $got) {
            throw "checksum mismatch for $archive; not installing"
        }

        Expand-Archive -Path "$tmp\$archive" -DestinationPath "$tmp\x"
        $version = & "$tmp\x\unsit.exe" --version
        if ($LASTEXITCODE -ne 0) {
            throw "the downloaded unsit.exe does not run on this system; not installing"
        }
        New-Item -ItemType Directory -Force -Path $dir | Out-Null
        try {
            Copy-Item -Force "$tmp\x\unsit.exe" "$dir\unsit.exe"
        } catch {
            throw "could not replace $dir\unsit.exe. If unsit is running, stop it from its tray menu and try again."
        }
    } finally {
        Remove-Item -Recurse -Force $tmp -ErrorAction SilentlyContinue
    }

    # Edit the registry value directly: [Environment]::SetEnvironmentVariable
    # would expand %VARIABLES% in the existing PATH and change its type.
    $key = [Microsoft.Win32.Registry]::CurrentUser.OpenSubKey('Environment', $true)
    $userPath = $key.GetValue('Path', '', 'DoNotExpandEnvironmentNames')
    if (($userPath -split ';') -notcontains $dir) {
        $newPath = if ($userPath) { $userPath.TrimEnd(';') + ";$dir" } else { $dir }
        $key.SetValue('Path', $newPath, 'ExpandString')
        $env:Path = "$env:Path;$dir"
        # Tell Explorer, so terminals opened from now on get the new PATH.
        Add-Type -Namespace Win32 -Name Env -MemberDefinition '[DllImport("user32.dll", CharSet = CharSet.Unicode)] public static extern IntPtr SendMessageTimeout(IntPtr hWnd, uint msg, UIntPtr wParam, string lParam, uint flags, uint timeout, out UIntPtr result);'
        $result = [UIntPtr]::Zero
        [void][Win32.Env]::SendMessageTimeout([IntPtr]0xffff, 0x1A, [UIntPtr]::Zero, 'Environment', 2, 5000, [ref]$result)
        Write-Host "Added $dir to your user PATH."
    }
    $key.Close()
    Write-Host "Installed $version. Run: unsit"
}
