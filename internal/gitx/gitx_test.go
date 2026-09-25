package gitx

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func runGit(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
	}
	return strings.TrimSpace(string(out))
}

func TestCatFileBatchReadsObjectsInOrder(t *testing.T) {
	dir := t.TempDir()
	runGit(t, dir, "init", "-q")
	runGit(t, dir, "config", "user.name", "Batch Reader")
	runGit(t, dir, "config", "user.email", "batch@example.invalid")
	var shas []string
	for _, body := range []string{"one", "two", "three"} {
		if err := os.WriteFile(filepath.Join(dir, "a.txt"), []byte(body+"\n"), 0644); err != nil {
			t.Fatal(err)
		}
		runGit(t, dir, "add", "a.txt")
		runGit(t, dir, "commit", "-q", "-m", body)
		shas = append(shas, runGit(t, dir, "rev-parse", "HEAD"))
	}
	repo, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	objects, err := repo.CatFileBatch(context.Background(), shas, "commit")
	if err != nil {
		t.Fatal(err)
	}
	if len(objects) != len(shas) {
		t.Fatalf("objects=%d want %d", len(objects), len(shas))
	}
	for i, body := range []string{"one", "two", "three"} {
		if !strings.HasSuffix(string(objects[i]), "\n\n"+body+"\n") {
			t.Fatalf("object %d out of order or wrong content", i)
		}
	}
}

func TestCatFileBatchHonorsCanceledContext(t *testing.T) {
	dir := t.TempDir()
	runGit(t, dir, "init", "-q")
	runGit(t, dir, "config", "user.name", "Batch Reader")
	runGit(t, dir, "config", "user.email", "batch@example.invalid")
	if err := os.WriteFile(filepath.Join(dir, "a.txt"), []byte("a\n"), 0644); err != nil {
		t.Fatal(err)
	}
	runGit(t, dir, "add", "a.txt")
	runGit(t, dir, "commit", "-q", "-m", "one")
	sha := runGit(t, dir, "rev-parse", "HEAD")
	repo, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := repo.CatFileBatch(ctx, []string{sha}, "commit"); err == nil {
		t.Fatal("expected canceled context error")
	}
}

func TestOpenAndCommandWrappers(t *testing.T) {
	dir := t.TempDir()
	runGit(t, dir, "init", "-q")
	runGit(t, dir, "config", "user.name", "Wrapper Test")
	runGit(t, dir, "config", "user.email", "wrapper@example.invalid")
	if err := os.WriteFile(filepath.Join(dir, "a.txt"), []byte("hello\n"), 0644); err != nil {
		t.Fatal(err)
	}
	runGit(t, dir, "add", "a.txt")
	runGit(t, dir, "commit", "-q", "-m", "initial")

	repo, err := OpenContext(context.Background(), dir)
	if err != nil {
		t.Fatal(err)
	}
	if repo.Bare || repo.Root == "" || repo.GitDir == "" {
		t.Fatalf("unexpected repo: %+v", repo)
	}
	out, err := repo.Run("rev-parse", "HEAD")
	if err != nil || strings.TrimSpace(string(out)) == "" {
		t.Fatalf("Run: output=%q err=%v", out, err)
	}
	out, err = repo.RunContext(context.Background(), "show", "-s", "--format=%s", "HEAD")
	if err != nil || strings.TrimSpace(string(out)) != "initial" {
		t.Fatalf("RunContext: output=%q err=%v", out, err)
	}
	if _, ok, err := repo.RunOptional("rev-parse", "--verify", "--quiet", "refs/heads/missing"); err != nil || ok {
		t.Fatalf("RunOptional missing: ok=%v err=%v", ok, err)
	}
	if _, ok, err := repo.RunOptionalContext(context.Background(), "rev-parse", "--verify", "--quiet", "refs/heads/missing"); err != nil || ok {
		t.Fatalf("RunOptionalContext missing: ok=%v err=%v", ok, err)
	}
	if out, err = repo.RunInput([]byte("payload\n"), "hash-object", "--stdin"); err != nil || strings.TrimSpace(string(out)) == "" {
		t.Fatalf("RunInput: output=%q err=%v", out, err)
	}
	if out, err = repo.RunInputContext(context.Background(), []byte("payload-2\n"), "hash-object", "--stdin"); err != nil || strings.TrimSpace(string(out)) == "" {
		t.Fatalf("RunInputContext: output=%q err=%v", out, err)
	}
	if out, err = run(dir, "rev-parse", "--is-inside-work-tree"); err != nil || strings.TrimSpace(string(out)) != "true" {
		t.Fatalf("run: output=%q err=%v", out, err)
	}
	if _, err := repo.Run("not-a-real-subcommand"); err == nil {
		t.Fatal("expected Git command error")
	}
}

func TestOpenBareAndInvalidRepositories(t *testing.T) {
	bareDir := filepath.Join(t.TempDir(), "repo.git")
	runGit(t, filepath.Dir(bareDir), "init", "--bare", "-q", bareDir)
	repo, err := Open(bareDir)
	if err != nil {
		t.Fatal(err)
	}
	if !repo.Bare || repo.Root != bareDir {
		t.Fatalf("unexpected bare repo: %+v", repo)
	}
	if _, err := Open(t.TempDir()); err == nil || !strings.Contains(err.Error(), "not a git repository") {
		t.Fatalf("expected non-repository error, got %v", err)
	}
}

func TestCatFileBatchInputValidation(t *testing.T) {
	dir := t.TempDir()
	runGit(t, dir, "init", "-q")
	repo, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	objects, err := repo.CatFileBatch(context.Background(), nil, "")
	if err != nil || objects != nil {
		t.Fatalf("empty batch: objects=%v err=%v", objects, err)
	}
	if _, err := repo.CatFileBatch(context.Background(), []string{"missing"}, "commit"); err == nil || !strings.Contains(err.Error(), "missing") {
		t.Fatalf("expected missing object error, got %v", err)
	}
}
