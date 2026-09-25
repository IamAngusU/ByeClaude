$ErrorActionPreference = 'Stop'
$repoRoot = [IO.Path]::GetFullPath((Join-Path $PSScriptRoot '..'))
$temporary = Join-Path ([IO.Path]::GetTempPath()) ('byeclaude-installer-test-' + [guid]::NewGuid().ToString('N'))
$assets = Join-Path $temporary 'assets'
$install = Join-Path $temporary 'install'
$version = 'v0.0.0-test'
$originalVersion = $env:BYECLAUDE_VERSION
$originalBase = $env:BYECLAUDE_DOWNLOAD_BASE
$originalInstall = $env:BYECLAUDE_INSTALL_DIR
$originalGOOS = $env:GOOS
$originalGOARCH = $env:GOARCH
$originalCGO = $env:CGO_ENABLED

try {
    New-Item -ItemType Directory -Path $assets, $install | Out-Null
    $arch = switch ([System.Runtime.InteropServices.RuntimeInformation]::OSArchitecture) {
        'X64' { 'amd64' }
        'Arm64' { 'arm64' }
        default { throw "Unsupported test architecture: $_" }
    }
    $assetName = "byeclaude_windows_${arch}.exe"
    $asset = Join-Path $assets $assetName
    $env:GOOS = 'windows'
    $env:GOARCH = $arch
    $env:CGO_ENABLED = '0'
    & go build -trimpath -buildvcs=false -ldflags "-s -w -X main.version=$version" -o $asset ./cmd/byeclaude
    if ($LASTEXITCODE -ne 0) { throw 'Test binary build failed.' }

    $hash = (Get-FileHash -LiteralPath $asset -Algorithm SHA256).Hash.ToLowerInvariant()
    [IO.File]::WriteAllText((Join-Path $assets 'SHA256SUMS.txt'), "$hash  $assetName`n", (New-Object Text.UTF8Encoding($false)))
    $env:BYECLAUDE_VERSION = $version
    $env:BYECLAUDE_DOWNLOAD_BASE = ([uri]$assets).AbsoluteUri.TrimEnd('/')
    $env:BYECLAUDE_INSTALL_DIR = $install
    & (Join-Path $repoRoot 'install.ps1')

    $installed = Join-Path $install 'byeclaude.exe'
    if (-not (Test-Path -LiteralPath $installed -PathType Leaf)) { throw 'Installer did not create byeclaude.exe.' }
    $reported = (& $installed version).Trim()
    if ($reported -ne "byeclaude $version") { throw "Installed binary reported unexpected version: $reported" }
    $installedHash = (Get-FileHash -LiteralPath $installed -Algorithm SHA256).Hash

    [IO.File]::WriteAllBytes($asset, [byte[]](1, 2, 3, 4))
    $rejected = $false
    try { & (Join-Path $repoRoot 'install.ps1') } catch { $rejected = $_.Exception.Message.Contains('Checksum mismatch') }
    if (-not $rejected) { throw 'Installer accepted a corrupted binary.' }
    if ((Get-FileHash -LiteralPath $installed -Algorithm SHA256).Hash -ne $installedHash) {
        throw 'Failed installer attempt replaced the existing binary.'
    }

    $env:BYECLAUDE_DOWNLOAD_BASE = $null
    $env:BYECLAUDE_VERSION = 'not-a-version'
    $invalidRejected = $false
    try { & (Join-Path $repoRoot 'install.ps1') } catch { $invalidRejected = $_.Exception.Message.Contains('semantic v-prefixed tag') }
    if (-not $invalidRejected) { throw 'Installer accepted an invalid version selector.' }
    Write-Host 'PowerShell installer positive and failure-path tests passed.' -ForegroundColor Green
}
finally {
    $env:BYECLAUDE_VERSION = $originalVersion
    $env:BYECLAUDE_DOWNLOAD_BASE = $originalBase
    $env:BYECLAUDE_INSTALL_DIR = $originalInstall
    $env:GOOS = $originalGOOS
    $env:GOARCH = $originalGOARCH
    $env:CGO_ENABLED = $originalCGO
    if (Test-Path -LiteralPath $temporary) {
        $resolved = [IO.Path]::GetFullPath($temporary)
        $prefix = [IO.Path]::GetFullPath([IO.Path]::GetTempPath()).TrimEnd([IO.Path]::DirectorySeparatorChar) + [IO.Path]::DirectorySeparatorChar
        if (-not $resolved.StartsWith($prefix, [StringComparison]::OrdinalIgnoreCase) -or
            -not ([IO.Path]::GetFileName($resolved) -match '^byeclaude-installer-test-[0-9a-f]{32}$')) {
            throw "Refusing to remove unexpected installer test directory: $resolved"
        }
        Remove-Item -LiteralPath $resolved -Recurse -Force
    }
}
