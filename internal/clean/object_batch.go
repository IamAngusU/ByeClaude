package clean

import (
	"context"
	"crypto/sha1" // #nosec G505 -- Git's object format, not a security primitive
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"hash"
	"os"
	"path/filepath"
	"strings"

	"github.com/IamAngusU/ByeClaude/internal/gitx"
)

// objectBatch computes object IDs in process so descendants can refer to
// rewritten parents immediately, then writes every staged object through one
// verified git hash-object process. This keeps Git as the object database
// writer without spawning one process per rewritten commit.
type objectBatch struct {
	repo       *gitx.Repo
	objectType string
	format     string
	dir        string
	names      []string
	expected   []string
}

func newObjectBatch(repo *gitx.Repo, objectType string) (*objectBatch, error) {
	formatOut, err := repo.Run("rev-parse", "--show-object-format")
	if err != nil {
		return nil, err
	}
	format := strings.TrimSpace(string(formatOut))
	if format != "sha1" && format != "sha256" {
		return nil, fmt.Errorf("unsupported Git object format %q", format)
	}
	return &objectBatch{repo: repo, objectType: objectType, format: format}, nil
}

func (b *objectBatch) Stage(raw []byte) (string, error) {
	if b.dir == "" {
		dir, err := os.MkdirTemp(b.repo.GitDir, "byeclaude-object-batch-")
		if err != nil {
			return "", fmt.Errorf("create object staging directory: %w", err)
		}
		b.dir = dir
	}
	name := fmt.Sprintf("%08d", len(b.names))
	if err := os.WriteFile(filepath.Join(b.dir, name), raw, 0600); err != nil {
		return "", fmt.Errorf("stage Git object: %w", err)
	}
	want := gitObjectID(b.format, b.objectType, raw)
	b.names = append(b.names, name)
	b.expected = append(b.expected, want)
	return want, nil
}

func (b *objectBatch) Flush(ctx context.Context) error {
	if len(b.names) == 0 {
		return nil
	}
	input := []byte(strings.Join(b.names, "\n") + "\n")
	out, err := b.repo.RunInputInDirContext(ctx, b.dir, input,
		"--git-dir="+b.repo.GitDir, "hash-object", "-t", b.objectType, "-w", "--stdin-paths", "--no-filters")
	if err != nil {
		return err
	}
	got := strings.Fields(string(out))
	if len(got) != len(b.expected) {
		return fmt.Errorf("Git wrote %d %s objects, expected %d", len(got), b.objectType, len(b.expected))
	}
	for i := range got {
		if got[i] != b.expected[i] {
			return fmt.Errorf("Git %s object hash mismatch at item %d: computed %s, wrote %s", b.objectType, i, b.expected[i], got[i])
		}
	}
	return nil
}

func (b *objectBatch) Close() error {
	if b.dir == "" {
		return nil
	}
	dir := b.dir
	if err := os.RemoveAll(dir); err != nil {
		return err
	}
	b.dir = ""
	return nil
}

func gitObjectID(format, objectType string, raw []byte) string {
	var h hash.Hash
	if format == "sha256" {
		h = sha256.New()
	} else {
		h = sha1.New() // #nosec G401 -- required for SHA-1-format Git repositories
	}
	_, _ = fmt.Fprintf(h, "%s %d%c", objectType, len(raw), byte(0))
	_, _ = h.Write(raw)
	return hex.EncodeToString(h.Sum(nil))
}
