[CmdletBinding()]
param()

$ErrorActionPreference = 'Stop'
$repo = 'IamAngusU/ByeClaude'

$arch = switch ([System.Runtime.InteropServices.RuntimeInformation]::OSArchitecture) {
    'X64' { 'amd64' }
    'Arm64' { 'arm64' }
    default { throw "Unsupported architecture: $_" }
}
$asset = "byeclaude_windows_${arch}.exe"
$version = if ($env:BYECLAUDE_VERSION) { $env:BYECLAUDE_VERSION.Trim() } else { 'latest' }

if ($version -ne 'latest' -and $version -notmatch '^v\d+\.\d+\.\d+(?:-[0-9A-Za-z][0-9A-Za-z.-]*)?$') {
    throw "BYECLAUDE_VERSION must be 'latest' or a semantic v-prefixed tag"
}

if ($env:BYECLAUDE_DOWNLOAD_BASE) {
    $base = $env:BYECLAUDE_DOWNLOAD_BASE.TrimEnd('/')
    if ($base -notmatch '^(https|file)://') {
        throw 'BYECLAUDE_DOWNLOAD_BASE must use https:// or file://'
    }
}
elseif ($version -eq 'latest') {
    $base = "https://github.com/$repo/releases/latest/download"
}
else {
    $base = "https://github.com/$repo/releases/download/$version"
}

function Receive-ByeClaudeFile {
    param(
        [Parameter(Mandatory = $true)][string]$Uri,
        [Parameter(Mandatory = $true)][string]$OutFile
    )
    if ($Uri.StartsWith('file://', [StringComparison]::OrdinalIgnoreCase)) {
        Copy-Item -LiteralPath ([uri]$Uri).LocalPath -Destination $OutFile
        return
    }
    Invoke-WebRequest -UseBasicParsing -Uri $Uri -OutFile $OutFile
}

$tmp = Join-Path ([System.IO.Path]::GetTempPath()) ("byeclaude-" + [guid]::NewGuid().ToString('N'))
New-Item -ItemType Directory -Path $tmp | Out-Null
try {
    $bin = Join-Path $tmp 'byeclaude.exe'
    $sums = Join-Path $tmp 'SHA256SUMS.txt'
    Receive-ByeClaudeFile -Uri "$base/$asset" -OutFile $bin
    Receive-ByeClaudeFile -Uri "$base/SHA256SUMS.txt" -OutFile $sums

    $escapedAsset = [regex]::Escape($asset)
    $line = Get-Content -LiteralPath $sums | Where-Object { $_ -match "^[0-9a-fA-F]{64}\s+\*?$escapedAsset$" } | Select-Object -First 1
    if (-not $line) { throw "Checksum entry not found for $asset" }
    $expected = ($line -split '\s+')[0].ToLowerInvariant()
    $actual = (Get-FileHash -LiteralPath $bin -Algorithm SHA256).Hash.ToLowerInvariant()
    if ($actual -ne $expected) { throw "Checksum mismatch for $asset" }

    $dest = if ($env:BYECLAUDE_INSTALL_DIR) {
        [IO.Path]::GetFullPath($env:BYECLAUDE_INSTALL_DIR)
    }
    else {
        Join-Path $env:LOCALAPPDATA 'Programs\ByeClaude'
    }
    New-Item -ItemType Directory -Force -Path $dest | Out-Null
    $target = Join-Path $dest 'byeclaude.exe'
    $stage = Join-Path $dest ('.byeclaude-' + [guid]::NewGuid().ToString('N') + '.exe')
    try {
        Copy-Item -LiteralPath $bin -Destination $stage
        & $stage version | Out-Null
        if ($LASTEXITCODE -ne 0) { throw 'Downloaded ByeClaude binary did not execute successfully' }
        Move-Item -LiteralPath $stage -Destination $target -Force
    }
    finally {
        Remove-Item -LiteralPath $stage -Force -ErrorAction SilentlyContinue
    }

    Write-Host "Installed byeclaude to $target"
    if (($env:PATH -split [IO.Path]::PathSeparator) -notcontains $dest) {
        Write-Host 'Add that directory to PATH if it is not already present.'
    }
}
finally {
    if (Test-Path -LiteralPath $tmp) {
        $resolved = [IO.Path]::GetFullPath($tmp)
        $tempRoot = [IO.Path]::GetFullPath([IO.Path]::GetTempPath()).TrimEnd([IO.Path]::DirectorySeparatorChar) + [IO.Path]::DirectorySeparatorChar
        if (-not $resolved.StartsWith($tempRoot, [StringComparison]::OrdinalIgnoreCase) -or
            -not ([IO.Path]::GetFileName($resolved) -match '^byeclaude-[0-9a-f]{32}$')) {
            throw "Refusing to remove unexpected temporary directory: $resolved"
        }
        Remove-Item -LiteralPath $resolved -Recurse -Force
    }
}
