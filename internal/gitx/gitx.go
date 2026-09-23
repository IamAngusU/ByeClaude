package gitx

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

type Repo struct {
	Root   string
	GitDir string
	Bare   bool
}

func Open(path string) (*Repo, error) {
	return OpenContext(context.Background(), path)
}

func OpenContext(ctx context.Context, path string) (*Repo, error) {
	if path == "" {
		path = "."
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return nil, err
	}
	bareOut, err := runContext(ctx, abs, "rev-parse", "--is-bare-repository")
	if err != nil {
		return nil, fmt.Errorf("not a git repository: %w", err)
	}
	bare := strings.TrimSpace(string(bareOut)) == "true"
	gitDirOut, err := runContext(ctx, abs, "rev-parse", "--absolute-git-dir")
	if err != nil {
		return nil, err
	}
	root := abs
	if !bare {
		rootOut, err := runContext(ctx, abs, "rev-parse", "--show-toplevel")
		if err != nil {
			return nil, err
		}
		root = strings.TrimSpace(string(rootOut))
	}
	return &Repo{Root: root, GitDir: strings.TrimSpace(string(gitDirOut)), Bare: bare}, nil
}
func (r *Repo) Run(args ...string) ([]byte, error) {
	return r.RunContext(context.Background(), args...)
}

func (r *Repo) RunContext(ctx context.Context, args ...string) ([]byte, error) {
	return runContext(ctx, r.Root, args...)
}

func (r *Repo) RunOptional(args ...string) ([]byte, bool, error) {
	return r.RunOptionalContext(context.Background(), args...)
}

func (r *Repo) RunOptionalContext(ctx context.Context, args ...string) ([]byte, bool, error) {
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = r.Root
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return nil, false, ctxErr
		}
		if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == 1 {
			return nil, false, nil
		}
		return nil, false, fmt.Errorf("git %s: %w\n%s", strings.Join(args, " "), err, strings.TrimSpace(stderr.String()))
	}
	return stdout.Bytes(), true, nil
}

func (r *Repo) RunInput(input []byte, args ...string) ([]byte, error) {
	return r.RunInputContext(context.Background(), input, args...)
}

func (r *Repo) RunInputContext(ctx context.Context, input []byte, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = r.Root
	cmd.Stdin = bytes.NewReader(input)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return nil, ctxErr
		}
		return nil, fmt.Errorf("git %s: %w\n%s", strings.Join(args, " "), err, strings.TrimSpace(stderr.String()))
	}
	return stdout.Bytes(), nil
}

// CatFileBatch reads Git objects in request order using one persistent
// git cat-file --batch process. This avoids spawning one Git process per
// commit during scans while still respecting cancellation through ctx.
func (r *Repo) CatFileBatch(ctx context.Context, objectNames []string, expectedType string) ([][]byte, error) {
	if len(objectNames) == 0 {
		return nil, nil
	}
	cmd := exec.CommandContext(ctx, "git", "cat-file", "--batch")
	cmd.Dir = r.Root
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, err
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Start(); err != nil {
		return nil, err
	}

	writeErr := make(chan error, 1)
	go func() {
		defer stdin.Close()
		w := bufio.NewWriter(stdin)
		for _, name := range objectNames {
			if _, err := fmt.Fprintln(w, name); err != nil {
				writeErr <- err
				return
			}
		}
		writeErr <- w.Flush()
	}()

	reader := bufio.NewReader(stdout)
	objects := make([][]byte, 0, len(objectNames))
	for i, requested := range objectNames {
		header, err := reader.ReadString('\n')
		if err != nil {
			_ = cmd.Process.Kill()
			_ = cmd.Wait()
			return nil, fmt.Errorf("cat-file batch header %d (%s): %w", i, requested, err)
		}
		fields := strings.Fields(header)
		if len(fields) == 2 && fields[1] == "missing" {
			_ = cmd.Process.Kill()
			_ = cmd.Wait()
			return nil, fmt.Errorf("git object %s is missing", requested)
		}
		if len(fields) != 3 {
			_ = cmd.Process.Kill()
			_ = cmd.Wait()
			return nil, fmt.Errorf("unexpected cat-file header for %s: %q", requested, strings.TrimSpace(header))
		}
		if expectedType != "" && fields[1] != expectedType {
			_ = cmd.Process.Kill()
			_ = cmd.Wait()
			return nil, fmt.Errorf("git object %s has type %s, want %s", requested, fields[1], expectedType)
		}
		size, err := strconv.ParseInt(fields[2], 10, 64)
		if err != nil || size < 0 {
			_ = cmd.Process.Kill()
			_ = cmd.Wait()
			return nil, fmt.Errorf("invalid cat-file size for %s: %q", requested, fields[2])
		}
		if size > int64(^uint(0)>>1) {
			_ = cmd.Process.Kill()
			_ = cmd.Wait()
			return nil, fmt.Errorf("git object %s is too large to read", requested)
		}
		data := make([]byte, int(size))
		if _, err := io.ReadFull(reader, data); err != nil {
			_ = cmd.Process.Kill()
			_ = cmd.Wait()
			return nil, fmt.Errorf("read git object %s: %w", requested, err)
		}
		separator, err := reader.ReadByte()
		if err != nil || separator != '\n' {
			_ = cmd.Process.Kill()
			_ = cmd.Wait()
			return nil, fmt.Errorf("invalid cat-file separator after %s", requested)
		}
		objects = append(objects, data)
	}

	if err := <-writeErr; err != nil {
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
		return nil, fmt.Errorf("write cat-file requests: %w", err)
	}
	if err := cmd.Wait(); err != nil {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return nil, ctxErr
		}
		return nil, fmt.Errorf("git cat-file --batch: %w\n%s", err, strings.TrimSpace(stderr.String()))
	}
	return objects, nil
}
func run(dir string, args ...string) ([]byte, error) {
	return runContext(context.Background(), dir, args...)
}

func runContext(ctx context.Context, dir string, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = dir
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return nil, ctxErr
		}
		return nil, fmt.Errorf("git %s: %w\n%s", strings.Join(args, " "), err, strings.TrimSpace(stderr.String()))
	}
	return stdout.Bytes(), nil
}
