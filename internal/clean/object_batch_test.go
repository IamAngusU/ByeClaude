package clean

import (
	"context"
	"os"
	"os/exec"
	"strings"
	"testing"

	"github.com/IamAngusU/ByeClaude/internal/gitx"
)

func TestObjectBatchWritesAndVerifiesGitObjects(t *testing.T) {
	dir := t.TempDir()
	if out, err := exec.Command("git", "init", "-q", dir).CombinedOutput(); err != nil {
		t.Fatalf("git init: %v\n%s", err, out)
	}
	repo, err := gitx.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	batch, err := newObjectBatch(repo, "blob")
	if err != nil {
		t.Fatal(err)
	}
	staging := ""
	t.Cleanup(func() { _ = batch.Close() })
	want, err := batch.Stage([]byte("first payload\n"))
	if err != nil {
		t.Fatal(err)
	}
	staging = batch.dir
	second, err := batch.Stage([]byte("second payload\n"))
	if err != nil {
		t.Fatal(err)
	}
	if want == second {
		t.Fatal("different objects received the same ID")
	}
	if err := batch.Flush(context.Background()); err != nil {
		t.Fatal(err)
	}
	out, err := repo.Run("cat-file", "blob", want)
	if err != nil || string(out) != "first payload\n" {
		t.Fatalf("stored object: %q, %v", out, err)
	}
	if err := batch.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(staging); !os.IsNotExist(err) {
		t.Fatalf("staging directory remains: %v", err)
	}
}

func TestGitObjectIDKnownSHA1(t *testing.T) {
	if got := gitObjectID("sha1", "blob", []byte("hello\n")); got != "ce013625030ba8dba906f756967f9e9ca394464a" {
		t.Fatalf("SHA-1 object ID = %s", got)
	}
	if got := gitObjectID("sha256", "blob", []byte("hello\n")); len(got) != 64 || strings.Trim(got, "0123456789abcdef") != "" {
		t.Fatalf("invalid SHA-256 object ID %q", got)
	}
}
