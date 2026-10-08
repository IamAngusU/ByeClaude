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
$originalState = $env:BYECLAUDE_STATE_DIR
$originalLocalAppData = $env:LOCALAPPDATA
$originalUserProfile = $env:USERPROFILE

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
    $env:BYECLAUDE_STATE_DIR = Join-Path $temporary 'state'
    & (Join-Path $repoRoot 'install.ps1') -NoPath

    $installed = Join-Path $install 'byeclaude.exe'
    if (-not (Test-Path -LiteralPath $installed -PathType Leaf)) { throw 'Installer did not create byeclaude.exe.' }
    $reported = (& $installed version).Trim()
    if ($reported -ne "byeclaude $version") { throw "Installed binary reported unexpected version: $reported" }
    $installedHash = (Get-FileHash -LiteralPath $installed -Algorithm SHA256).Hash

    # Optional PATH failure must leave a working binary and preserve user PATH.
    $beforeUserPath = [Environment]::GetEnvironmentVariable('Path', 'User')
    $blockedState = Join-Path $temporary 'blocked-state'
    [IO.File]::WriteAllText($blockedState, 'keep')
    $env:BYECLAUDE_STATE_DIR = $blockedState
    $pathFailureOutput = & (Join-Path $repoRoot 'install.ps1') *>&1 | Out-String
    if ($pathFailureOutput -notmatch 'PATH setup deferred') { throw 'Expected a nonfatal PATH setup diagnostic.' }
    if ((& $installed version).Trim() -ne "byeclaude $version") { throw 'PATH failure broke the installed binary.' }
    if ([Environment]::GetEnvironmentVariable('Path', 'User') -ne $beforeUserPath) { throw 'Installer test changed real user PATH.' }

    # A blocked custom destination falls back without elevation.
    $env:BYECLAUDE_STATE_DIR = Join-Path $temporary 'state'
    $blockedDestination = Join-Path $temporary 'blocked-destination'
    [IO.File]::WriteAllText($blockedDestination, 'keep')
    $env:BYECLAUDE_INSTALL_DIR = $blockedDestination
    $env:LOCALAPPDATA = Join-Path $temporary 'fallback-local'
    $env:USERPROFILE = Join-Path $temporary 'fallback-user'
    & (Join-Path $repoRoot 'install.ps1') -NoPath
    $fallbackBinary = Join-Path $env:LOCALAPPDATA 'Programs\ByeClaude\byeclaude.exe'
    if ((& $fallbackBinary version).Trim() -ne "byeclaude $version") { throw 'User-directory fallback failed.' }
    if ([IO.File]::ReadAllText($blockedDestination) -ne 'keep') { throw 'Blocked destination was replaced.' }
    $env:BYECLAUDE_INSTALL_DIR = $install
    $env:LOCALAPPDATA = $originalLocalAppData
    $env:USERPROFILE = $originalUserProfile

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
    $env:BYECLAUDE_STATE_DIR = $originalState
    $env:LOCALAPPDATA = $originalLocalAppData
    $env:USERPROFILE = $originalUserProfile
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
