$ErrorActionPreference = 'Stop'
$repoRoot = [IO.Path]::GetFullPath((Join-Path $PSScriptRoot '..'))
$temporary = Join-Path ([IO.Path]::GetTempPath()) ('byeclaude-workflow-' + [guid]::NewGuid().ToString('N'))
$workspace = Join-Path $temporary "work with space's"
$binaryDirectory = Join-Path $temporary "tool with space's"
$remote = Join-Path $temporary 'remote.git'
$binary = Join-Path $binaryDirectory 'byeclaude.exe'
$oldAuthorName = $env:GIT_AUTHOR_NAME
$oldAuthorEmail = $env:GIT_AUTHOR_EMAIL
$oldGlobalConfig = $env:GIT_CONFIG_GLOBAL
$oldNoSystem = $env:GIT_CONFIG_NOSYSTEM

function Git-OK {
    & git @args
    if ($LASTEXITCODE -ne 0) { throw "Git failed: $args" }
}
function Bye-OK {
    & $binary @args
    if ($LASTEXITCODE -ne 0) { throw "ByeClaude failed: $args" }
}

try {
    New-Item -ItemType Directory -Path $workspace, $binaryDirectory | Out-Null
    $globalConfig = Join-Path $temporary 'empty.gitconfig'
    [IO.File]::WriteAllText($globalConfig, '')
    $env:GIT_CONFIG_GLOBAL = $globalConfig
    $env:GIT_CONFIG_NOSYSTEM = '1'
    $env:GIT_AUTHOR_NAME = $null
    $env:GIT_AUTHOR_EMAIL = $null
    Push-Location $repoRoot
    try {
        & go build -trimpath -buildvcs=false -o $binary ./cmd/byeclaude
        if ($LASTEXITCODE -ne 0) { throw 'Build failed.' }
    }
    finally { Pop-Location }
    Push-Location $workspace
    try {
        Git-OK init -q
        Git-OK config user.name 'Workflow Human'
        Git-OK config user.email 'human@example.org'
        [IO.File]::WriteAllText((Join-Path $workspace 'keep.txt'), "Preserve these bytes.`n")
        Git-OK add keep.txt
        Git-OK commit -q -m "Initial`n`nCo-authored-by: Claude <noreply@anthropic.com>`nCo-authored-by: Helper <helper@example.org>`nCo-authored-by: Human <human@example.org>"
        $original = (Git-OK rev-parse HEAD).Trim()
        $tree = (Git-OK rev-parse 'HEAD^{tree}').Trim()
        Bye-OK blacklist add --id helper --email helper@example.org
        Bye-OK plan
        if ((Git-OK rev-parse HEAD).Trim() -ne $original) { throw 'Plan changed history.' }
        $cleanReport = (Bye-OK clean --apply --json) | ConvertFrom-Json
        if ((Git-OK rev-parse 'HEAD^{tree}').Trim() -ne $tree) { throw 'Cleanup changed files.' }
        Bye-OK check --include-identities
        Bye-OK restore --backup $cleanReport.backup --apply
        if ((Git-OK rev-parse HEAD).Trim() -ne $original) { throw 'Restore did not recover original history.' }
        Bye-OK clean --apply
        Bye-OK setup --apply
        Bye-OK setup --apply
        Bye-OK doctor
        Bye-OK blacklist add --id later-bot --email later@example.org
        # An actual Git invocation must execute the generated shell hook on Windows.
        Git-OK commit --allow-empty -q -m "Hook check`n`nCo-authored-by: Claude <noreply@anthropic.com>`nCo-authored-by: Helper <helper@example.org>`nCo-authored-by: Later <later@example.org>`nCo-authored-by: Human <human@example.org>"
        $message = (Git-OK log -1 --format=%B) -join "`n"
        if ($message.Contains('anthropic.com') -or $message.Contains('helper@example.org') -or $message.Contains('later@example.org') -or -not $message.Contains('human@example.org')) {
            throw 'Native commit-msg hook failed to preserve only the human attribution.'
        }
        Git-OK init --bare -q $remote
        Git-OK remote add origin $remote
        Git-OK push -q origin HEAD:refs/heads/main
        $published = (Git-OK --git-dir=$remote rev-parse refs/heads/main).Trim()
        $env:GIT_AUTHOR_NAME = 'Claude'
        $env:GIT_AUTHOR_EMAIL = 'noreply@anthropic.com'
        Git-OK commit --allow-empty -q -m 'Blocked author fixture'
        $env:GIT_AUTHOR_NAME = $null
        $env:GIT_AUTHOR_EMAIL = $null
        & git push -q origin HEAD:refs/heads/main 2>&1 | ForEach-Object { Write-Host $_ }
        if ($LASTEXITCODE -eq 0) { throw 'pre-push accepted a blocked author.' }
        if ((Git-OK --git-dir=$remote rev-parse refs/heads/main).Trim() -ne $published) { throw 'Rejected push moved remote history.' }
        Bye-OK clean --author-from-git --apply
        Git-OK push -q origin HEAD:refs/heads/main
        Bye-OK check --include-identities
        Write-Host 'Windows workflow passed: blacklist, preview, tree preservation, restore, real Git hooks, rejected push and author correction.' -ForegroundColor Green
    }
    finally { Pop-Location }
}
finally {
    $env:GIT_AUTHOR_NAME = $oldAuthorName
    $env:GIT_AUTHOR_EMAIL = $oldAuthorEmail
    $env:GIT_CONFIG_GLOBAL = $oldGlobalConfig
    $env:GIT_CONFIG_NOSYSTEM = $oldNoSystem
    if (Test-Path -LiteralPath $temporary) {
        $resolved = [IO.Path]::GetFullPath($temporary)
        $prefix = [IO.Path]::GetFullPath([IO.Path]::GetTempPath()).TrimEnd([IO.Path]::DirectorySeparatorChar) + [IO.Path]::DirectorySeparatorChar
        if (-not $resolved.StartsWith($prefix, [StringComparison]::OrdinalIgnoreCase) -or
            -not ([IO.Path]::GetFileName($resolved) -match '^byeclaude-workflow-[0-9a-f]{32}$')) {
            throw "Refusing to remove unexpected test directory: $resolved"
        }
        Remove-Item -LiteralPath $resolved -Recurse -Force
    }
}
