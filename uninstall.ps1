# Removes what install.ps1 set up: %LOCALAPPDATA%\Programs\unsit and its
# user PATH entry, plus the Startup shortcut from the README if it points
# there. Stops unsit first if it is running from that folder.
#
#   irm https://raw.githubusercontent.com/marcel290820/unsit/main/uninstall.ps1 | iex

& {
    $ErrorActionPreference = 'Stop'
    $dir = Join-Path $env:LOCALAPPDATA 'Programs\unsit'

    # Paths are compared and used literally: a user name can hold [ or ],
    # which -like and -Path read as wildcards.
    function Inside($path) { $path -and $path.StartsWith("$dir\", [StringComparison]::OrdinalIgnoreCase) }

    $running = Get-Process unsit -ErrorAction SilentlyContinue | Where-Object { Inside $_.Path }
    if ($running) {
        $running | Stop-Process -Force
        $running | Wait-Process -Timeout 10
    }
    if (Test-Path -LiteralPath $dir) {
        Remove-Item -Recurse -Force -LiteralPath $dir
        Write-Host "Removed $dir."
    }

    $startup = Join-Path ([Environment]::GetFolderPath('Startup')) 'unsit.lnk'
    if ((Test-Path -LiteralPath $startup) -and (Inside (New-Object -ComObject WScript.Shell).CreateShortcut($startup).TargetPath)) {
        Remove-Item -LiteralPath $startup
        Write-Host "Removed the Startup shortcut."
    }

    # Gone at sign-out anyway; removed now so Settings stops listing unsit.
    [Microsoft.Win32.Registry]::CurrentUser.DeleteSubKeyTree('Software\Classes\AppUserModelId\unsit', $false)

    # Edit the registry value directly, as install.ps1 does.
    $key = [Microsoft.Win32.Registry]::CurrentUser.OpenSubKey('Environment', $true)
    $userPath = $key.GetValue('Path', '', 'DoNotExpandEnvironmentNames')
    $entries = $userPath -split ';'
    if ($entries -contains $dir) {
        $key.SetValue('Path', (($entries | Where-Object { $_ -and $_ -ne $dir }) -join ';'), 'ExpandString')
        Add-Type -Namespace Win32 -Name Env -MemberDefinition '[DllImport("user32.dll", CharSet = CharSet.Unicode)] public static extern IntPtr SendMessageTimeout(IntPtr hWnd, uint msg, UIntPtr wParam, string lParam, uint flags, uint timeout, out UIntPtr result);'
        $result = [UIntPtr]::Zero
        [void][Win32.Env]::SendMessageTimeout([IntPtr]0xffff, 0x1A, [UIntPtr]::Zero, 'Environment', 2, 5000, [ref]$result)
        Write-Host "Removed $dir from your user PATH."
    }
    $key.Close()
    Write-Host "unsit is uninstalled."
}
