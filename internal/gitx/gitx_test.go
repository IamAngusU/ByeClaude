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
