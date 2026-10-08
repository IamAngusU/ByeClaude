[CmdletBinding()]
param([switch]$AddToPath, [switch]$NoPath, [switch]$NoStart)

$ErrorActionPreference = 'Stop'
$repo = 'IamAngusU/ByeClaude'

$arch = switch ([System.Runtime.InteropServices.RuntimeInformation]::OSArchitecture) {
    'X64' { 'amd64' }
    'Arm64' { 'arm64' }
    default { throw "Unsupported architecture: $_" }
}
$asset = "byeclaude_windows_${arch}.exe"
$version = if ($env:BYECLAUDE_VERSION) { $env:BYECLAUDE_VERSION.Trim() } else { 'v0.1.0-alpha.6' }

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

    $destinations = @()
    if ($env:BYECLAUDE_INSTALL_DIR) { $destinations += [IO.Path]::GetFullPath($env:BYECLAUDE_INSTALL_DIR) }
    if ($env:LOCALAPPDATA) { $destinations += Join-Path $env:LOCALAPPDATA 'Programs\ByeClaude' }
    if ($env:USERPROFILE) { $destinations += Join-Path $env:USERPROFILE '.local\bin\ByeClaude' }
    $stage = $null
    foreach ($candidate in ($destinations | Select-Object -Unique)) {
        $candidateStage = Join-Path $candidate ('.byeclaude-' + [guid]::NewGuid().ToString('N') + '.exe')
        try {
            New-Item -ItemType Directory -Force -Path $candidate | Out-Null
            Copy-Item -LiteralPath $bin -Destination $candidateStage
            $dest = $candidate
            $stage = $candidateStage
            break
        }
        catch {
            Remove-Item -LiteralPath $candidateStage -Force -ErrorAction SilentlyContinue
            Write-Host "Cannot write to $candidate. Trying a user-owned folder; no administrator access is requested."
        }
    }
    if (-not $stage) { throw 'No writable installation folder was found. Choose a writable BYECLAUDE_INSTALL_DIR and retry; no existing installation was replaced.' }
    $target = Join-Path $dest 'byeclaude.exe'
    try {
        & $stage version | Out-Null
        if ($LASTEXITCODE -ne 0) { throw 'Downloaded ByeClaude binary did not execute successfully' }
        Move-Item -LiteralPath $stage -Destination $target -Force
    }
    finally {
        Remove-Item -LiteralPath $stage -Force -ErrorAction SilentlyContinue
    }

    Write-Host "Installed byeclaude to $target"
    $pathAction = if ($NoPath -or $env:BYECLAUDE_NO_PATH -eq '1') { 'skip' } else { 'setup' }
    try {
        & $target path $pathAction
        if ($LASTEXITCODE -ne 0) { throw 'Optional PATH setup did not finish' }
    } catch {
        Write-Host "PATH setup can wait. ByeClaude is installed at $target."
        Write-Host "Start it by its full path; run 'path setup' to retry."
    }
    Write-Host 'Local activity counters stay on this computer. Run byeclaude metrics to view them, or metrics off to disable.'
    Write-Host 'Powered by angusu.de | Angus Uelsmann'
    $interactive = $false
    try { $interactive = -not [Console]::IsInputRedirected -and -not [Console]::IsOutputRedirected } catch { }
    $inCI = $env:CI -and $env:CI -notin @('false', '0')
    if (-not $NoStart -and $env:BYECLAUDE_NO_START -ne '1' -and $interactive -and -not $inCI) {
        if (Get-Command git -ErrorAction SilentlyContinue) {
            Write-Host 'Opening guided setup. History and hooks change only after your confirmation.'
            try {
                & $target guide
                if ($LASTEXITCODE -ne 0) { throw 'Guided setup did not finish' }
            } catch { Write-Host "Setup can wait. Start it again with: & '$target' guide" }
        } else {
            Write-Host 'ByeClaude is installed. Install Git from https://git-scm.com/downloads, then run byeclaude.'
        }
    } else {
        Write-Host 'Next: open a new terminal and run byeclaude for guided setup.'
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
        Remove-Item -LiteralPath $resolved -Recurse -Force -ErrorAction SilentlyContinue
    }
}
