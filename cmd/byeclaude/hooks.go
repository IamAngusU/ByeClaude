package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/IamAngusU/ByeClaude/internal/gitx"
)

func resolvedGitHookPath(repo *gitx.Repo, hookName string) (string, error) {
	if hookName != "commit-msg" && hookName != "pre-push" {
		return "", fmt.Errorf("unsupported hook: %q", hookName)
	}
	// rev-parse honors linked worktrees and Git's common directory layout.
	// Unlike <gitdir>/hooks this resolves the hooks directory Git actually uses.
	out, err := repo.Run("rev-parse", "--git-path", "hooks/"+hookName)
	if err != nil {
		return "", fmt.Errorf("resolve Git hook path: %w", err)
	}
	path := strings.TrimSpace(string(out))
	if path == "" {
		return "", fmt.Errorf("Git returned an empty hook path")
	}
	if !filepath.IsAbs(path) {
		path = filepath.Join(repo.Root, path)
	}
	return filepath.Clean(path), nil
}

func ownedByeClaudeHook(name string, content []byte) bool {
	// Preserve existing ByeClaude-installed hook format. A marker on its own
	// must never authorize deleting someone else's hook.
	s := string(content)
	if !strings.HasPrefix(s, "#!/bin/sh\n# Installed by ByeClaude.\nexec '") ||
		!strings.HasSuffix(s, "\n") ||
		strings.Count(s, "\n") != 3 {
		return false
	}
	needle := " hook-filter"
	end := " \"$1\"\n"
	if name == "pre-push" {
		needle = " pre-push-filter"
		end = " \"$@\"\n"
	}
	return strings.Contains(s, needle) && strings.HasSuffix(s, end)
}

func ephemeralGoExecutable(path string) bool {
	normalized := filepath.ToSlash(path)
	base := strings.ToLower(filepath.Base(path))
	// "go run" creates disposable go-build.../exe/byeclaude. Hooks pointing
	// at it will break as soon as go run exits. Tests use *.test executables.
	if strings.HasSuffix(base, ".test") || strings.HasSuffix(base, ".test.exe") {
		return false
	}
	return strings.Contains(normalized, "/go-build") && strings.Contains(normalized, "/exe/")
}

type inspectedHook struct {
	Name   string
	Path   string
	Status string
}

func inspectHook(repo *gitx.Repo, name string) (inspectedHook, error) {
	result := inspectedHook{Name: name, Status: "missing"}
	path, err := resolvedGitHookPath(repo, name)
	if err != nil {
		return result, err
	}
	result.Path = path
	info, err := os.Lstat(path)
	if os.IsNotExist(err) {
		return result, nil
	}
	if err != nil {
		return result, err
	}
	if !info.Mode().IsRegular() {
		result.Status = "conflict"
		return result, nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return result, err
	}
	if !ownedByeClaudeHook(name, data) {
		result.Status = "conflict"
		return result, nil
	}
	if info.Mode()&0111 == 0 {
		// Git for Windows may handle file modes differently, but on Unix
		// this is unusable. Never claim a hook works when it cannot execute.
		result.Status = "not_executable"
		return result, nil
	}
	result.Status = "installed"
	return result, nil
}

func effectiveHooksPath(repo *gitx.Repo) (string, error) {
	out, found, err := repo.RunOptional("config", "--path", "--get", "core.hooksPath")
	if err != nil {
		return "", err
	}
	if found {
		return strings.TrimSpace(string(out)), nil
	}
	return "", nil
}
