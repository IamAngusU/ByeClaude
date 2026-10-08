package pathsetup

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"syscall"
	"time"
)

// The directory is data in the environment, never interpolated into script code.
const userPathScript = `$ErrorActionPreference = 'Stop'
$target = $env:BYECLAUDE_PATH_TARGET
$path = [Environment]::GetEnvironmentVariable('Path', 'User')
$present = $false
foreach ($entry in ($path -split ';')) {
    if ([Environment]::ExpandEnvironmentVariables($entry.Trim('"').TrimEnd('\','/')).Equals($target.TrimEnd('\','/'), [StringComparison]::OrdinalIgnoreCase)) { $present = $true }
}
if (-not $present) {
    $updated = if ([string]::IsNullOrEmpty($path)) { $target } else { $path.TrimEnd(';') + ';' + $target }
    [Environment]::SetEnvironmentVariable('Path', $updated, 'User')
    if ([Environment]::GetEnvironmentVariable('Path', 'User') -ne $updated) { throw 'User PATH did not persist' }
}`

func configureUserPath(dir string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "powershell.exe", "-NoLogo", "-NoProfile", "-NonInteractive", "-Command", userPathScript)
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	cmd.Env = append(os.Environ(), "BYECLAUDE_PATH_TARGET="+dir)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("Windows could not save your user PATH (permission, policy, or PowerShell unavailable): %w", err)
	}
	return nil
}
