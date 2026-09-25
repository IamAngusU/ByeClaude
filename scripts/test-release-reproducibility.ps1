param(
    [ValidatePattern('^v\d+\.\d+\.\d+(?:-[0-9A-Za-z][0-9A-Za-z.-]*)?$')]
    [string]$Version = 'v0.1.0-alpha.1'
)

$ErrorActionPreference = 'Stop'
$repoRoot = [IO.Path]::GetFullPath((Join-Path $PSScriptRoot '..'))
$temporary = Join-Path ([IO.Path]::GetTempPath()) ('byeclaude-repro-' + [guid]::NewGuid().ToString('N'))
$first = Join-Path $temporary 'first'
$second = Join-Path $temporary 'second'
$originalGOOS = $env:GOOS
$originalGOARCH = $env:GOARCH
$originalCGO = $env:CGO_ENABLED

function Build-ReleaseSet([string]$Destination) {
    New-Item -ItemType Directory -Path $Destination | Out-Null
    $targets = @(
        @{ OS = 'linux'; Arch = 'amd64'; Extension = '' },
        @{ OS = 'linux'; Arch = 'arm64'; Extension = '' },
        @{ OS = 'darwin'; Arch = 'amd64'; Extension = '' },
        @{ OS = 'darwin'; Arch = 'arm64'; Extension = '' },
        @{ OS = 'windows'; Arch = 'amd64'; Extension = '.exe' },
        @{ OS = 'windows'; Arch = 'arm64'; Extension = '.exe' }
    )
    foreach ($target in $targets) {
        $env:GOOS = $target.OS
        $env:GOARCH = $target.Arch
        $env:CGO_ENABLED = '0'
        $name = "byeclaude_$($target.OS)_$($target.Arch)$($target.Extension)"
        & go build -trimpath -buildvcs=false -ldflags "-s -w -X main.version=$Version" -o (Join-Path $Destination $name) ./cmd/byeclaude
        if ($LASTEXITCODE -ne 0) { throw "Build failed for $($target.OS)/$($target.Arch)." }
    }
}

try {
    New-Item -ItemType Directory -Path $temporary | Out-Null
    Push-Location $repoRoot
    try {
        Build-ReleaseSet $first
        Build-ReleaseSet $second
        $commit = (& git rev-parse HEAD).Trim()
        $epoch = [long]((& git show -s --format=%ct HEAD).Trim())
        & (Join-Path $PSScriptRoot 'build-release-metadata.ps1') -Version $Version -ArtifactsDirectory $first -Commit $commit -SourceDateEpoch $epoch
        & (Join-Path $PSScriptRoot 'build-release-metadata.ps1') -Version $Version -ArtifactsDirectory $second -Commit $commit -SourceDateEpoch $epoch
    }
    finally {
        Pop-Location
    }

    $firstFiles = @(Get-ChildItem -LiteralPath $first -File | Sort-Object Name)
    $secondFiles = @(Get-ChildItem -LiteralPath $second -File | Sort-Object Name)
    if ($firstFiles.Count -ne 9 -or $secondFiles.Count -ne 9) { throw 'Expected six binaries, SBOM, provenance and checksums in each build.' }
    for ($i = 0; $i -lt $firstFiles.Count; $i++) {
        if ($firstFiles[$i].Name -ne $secondFiles[$i].Name) { throw 'Release file sets differ.' }
        $a = (Get-FileHash -LiteralPath $firstFiles[$i].FullName -Algorithm SHA256).Hash
        $b = (Get-FileHash -LiteralPath $secondFiles[$i].FullName -Algorithm SHA256).Hash
        if ($a -ne $b) { throw "Release binary is not reproducible: $($firstFiles[$i].Name)" }
    }
    Write-Host 'All release binaries and metadata are byte-identical across two builds.' -ForegroundColor Green
}
finally {
    $env:GOOS = $originalGOOS
    $env:GOARCH = $originalGOARCH
    $env:CGO_ENABLED = $originalCGO
    if (Test-Path -LiteralPath $temporary) {
        $resolved = [IO.Path]::GetFullPath($temporary)
        $prefix = [IO.Path]::GetFullPath([IO.Path]::GetTempPath()).TrimEnd([IO.Path]::DirectorySeparatorChar) + [IO.Path]::DirectorySeparatorChar
        if (-not $resolved.StartsWith($prefix, [StringComparison]::OrdinalIgnoreCase) -or
            -not ([IO.Path]::GetFileName($resolved) -match '^byeclaude-repro-[0-9a-f]{32}$')) {
            throw "Refusing to remove unexpected reproducibility directory: $resolved"
        }
        Remove-Item -LiteralPath $resolved -Recurse -Force
    }
}
