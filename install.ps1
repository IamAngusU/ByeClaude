$ErrorActionPreference = 'Stop'
$repo = 'IamAngusU/ByeClaude'
$arch = switch ([System.Runtime.InteropServices.RuntimeInformation]::OSArchitecture) {
    'X64' { 'amd64' }
    'Arm64' { 'arm64' }
    default { throw "Unsupported architecture: $_" }
}
$asset = "byeclaude_windows_${arch}.exe"
$base = "https://github.com/$repo/releases/latest/download"
$tmp = Join-Path ([System.IO.Path]::GetTempPath()) ("byeclaude-" + [guid]::NewGuid())
New-Item -ItemType Directory -Path $tmp | Out-Null
try {
    $bin = Join-Path $tmp 'byeclaude.exe'
    $sums = Join-Path $tmp 'SHA256SUMS.txt'
    Invoke-WebRequest "$base/$asset" -OutFile $bin
    Invoke-WebRequest "$base/SHA256SUMS.txt" -OutFile $sums
    $line = Get-Content $sums | Where-Object { $_ -match "\s$([regex]::Escape($asset))$" } | Select-Object -First 1
    if (-not $line) { throw 'Checksum entry not found' }
    $expected = ($line -split '\s+')[0].ToLowerInvariant()
    $actual = (Get-FileHash -Algorithm SHA256 $bin).Hash.ToLowerInvariant()
    if ($actual -ne $expected) { throw 'Checksum mismatch' }
    $dest = if ($env:BYECLAUDE_INSTALL_DIR) { $env:BYECLAUDE_INSTALL_DIR } else { Join-Path $env:LOCALAPPDATA 'Programs\ByeClaude' }
    New-Item -ItemType Directory -Force -Path $dest | Out-Null
    Copy-Item $bin (Join-Path $dest 'byeclaude.exe') -Force
    Write-Host "Installed byeclaude to $dest\byeclaude.exe"
    Write-Host "Add that directory to PATH if it is not already present."
}
finally {
    Remove-Item -Recurse -Force $tmp -ErrorAction SilentlyContinue
}
