param(
    [Parameter(Mandatory = $true)][ValidatePattern('^v\d+\.\d+\.\d+(?:-[0-9A-Za-z][0-9A-Za-z.-]*)?$')][string]$Version,
    [Parameter(Mandatory = $true)][string]$ArtifactsDirectory,
    [Parameter(Mandatory = $true)][ValidatePattern('^[0-9a-fA-F]{40,64}$')][string]$Commit,
    [Parameter(Mandatory = $true)][long]$SourceDateEpoch
)

$ErrorActionPreference = 'Stop'
$repoRoot = [IO.Path]::GetFullPath((Join-Path $PSScriptRoot '..'))
$artifacts = [IO.Path]::GetFullPath($ArtifactsDirectory)
if (-not (Test-Path -LiteralPath $artifacts -PathType Container)) { throw "Artifacts directory not found: $artifacts" }
$expected = @(
    'byeclaude_linux_amd64', 'byeclaude_linux_arm64',
    'byeclaude_darwin_amd64', 'byeclaude_darwin_arm64',
    'byeclaude_windows_amd64.exe', 'byeclaude_windows_arm64.exe'
)
foreach ($name in $expected) {
    if (-not (Test-Path -LiteralPath (Join-Path $artifacts $name) -PathType Leaf)) { throw "Missing release binary: $name" }
}

$originalGOOS = $env:GOOS
$originalGOARCH = $env:GOARCH
$originalCGO = $env:CGO_ENABLED
try {
    $env:GOOS = $null
    $env:GOARCH = $null
    $env:CGO_ENABLED = $null
    Push-Location $repoRoot
    try {
        & go run ./scripts/release-sbom --binary (Join-Path $artifacts 'byeclaude_linux_amd64') --version $Version --commit $Commit --epoch $SourceDateEpoch --output (Join-Path $artifacts 'SBOM.cdx.json')
        if ($LASTEXITCODE -ne 0) { throw 'SBOM generation failed.' }
    }
    finally {
        Pop-Location
    }
}
finally {
    $env:GOOS = $originalGOOS
    $env:GOARCH = $originalGOARCH
    $env:CGO_ENABLED = $originalCGO
}

$subjects = @(Get-ChildItem -LiteralPath $artifacts -File | Where-Object { $_.Name -notin @('SHA256SUMS.txt', 'BUILD-PROVENANCE.json') } | Sort-Object Name | ForEach-Object {
    [ordered]@{
        name = $_.Name
        sha256 = (Get-FileHash -LiteralPath $_.FullName -Algorithm SHA256).Hash.ToLowerInvariant()
        size_bytes = $_.Length
    }
})
$record = [ordered]@{
    schema_version = 1
    kind = 'unsigned-build-record'
    release = $Version
    source_repository = 'https://github.com/IamAngusU/ByeClaude'
    source_commit = $Commit.ToLowerInvariant()
    source_date_epoch = $SourceDateEpoch
    builder = [ordered]@{ tool = 'scripts/build-release-metadata.ps1'; go = ((& go version) -join ' ').Trim() }
    reproducibility = [ordered]@{ trimpath = $true; buildvcs = $false; cgo_enabled = $false; double_build_verified_in_ci = $true }
    signature_status = 'unsigned'
    subjects = $subjects
}
$utf8 = New-Object Text.UTF8Encoding($false)
$json = ($record | ConvertTo-Json -Depth 8) -replace "`r`n", "`n"
[IO.File]::WriteAllText((Join-Path $artifacts 'BUILD-PROVENANCE.json'), "$json`n", $utf8)

$checksumLines = @(Get-ChildItem -LiteralPath $artifacts -File | Where-Object Name -ne 'SHA256SUMS.txt' | Sort-Object Name | ForEach-Object {
    "$((Get-FileHash -LiteralPath $_.FullName -Algorithm SHA256).Hash.ToLowerInvariant())  $($_.Name)"
})
[IO.File]::WriteAllText((Join-Path $artifacts 'SHA256SUMS.txt'), (($checksumLines -join "`n") + "`n"), $utf8)
Write-Host "Release metadata generated for $($expected.Count) binaries." -ForegroundColor Green
