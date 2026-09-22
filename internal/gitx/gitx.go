package gitx

import (
	"bytes"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
)

type Repo struct {
	Root   string
	GitDir string
	Bare   bool
}

func Open(path string) (*Repo, error) {
	if path == "" {
		path = "."
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return nil, err
	}
	bareOut, err := run(abs, "rev-parse", "--is-bare-repository")
	if err != nil {
		return nil, fmt.Errorf("not a git repository: %w", err)
	}
	bare := strings.TrimSpace(string(bareOut)) == "true"
	gitDirOut, err := run(abs, "rev-parse", "--absolute-git-dir")
	if err != nil {
		return nil, err
	}
	root := abs
	if !bare {
		rootOut, err := run(abs, "rev-parse", "--show-toplevel")
		if err != nil {
			return nil, err
		}
		root = strings.TrimSpace(string(rootOut))
	}
	return &Repo{Root: root, GitDir: strings.TrimSpace(string(gitDirOut)), Bare: bare}, nil
}

func (r *Repo) Run(args ...string) ([]byte, error) { return run(r.Root, args...) }

func (r *Repo) RunOptional(args ...string) ([]byte, bool, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = r.Root
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == 1 {
			return nil, false, nil
		}
		return nil, false, fmt.Errorf("git %s: %w\n%s", strings.Join(args, " "), err, strings.TrimSpace(stderr.String()))
	}
	return stdout.Bytes(), true, nil
}

func (r *Repo) RunInput(input []byte, args ...string) ([]byte, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = r.Root
	cmd.Stdin = bytes.NewReader(input)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("git %s: %w\n%s", strings.Join(args, " "), err, strings.TrimSpace(stderr.String()))
	}
	return stdout.Bytes(), nil
}

func run(dir string, args ...string) ([]byte, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("git %s: %w\n%s", strings.Join(args, " "), err, strings.TrimSpace(stderr.String()))
	}
	return stdout.Bytes(), nil
}
