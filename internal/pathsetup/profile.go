package pathsetup

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

const beginMarker = "# >>> ByeClaude PATH >>>"
const endMarker = "# <<< ByeClaude PATH <<<"

func profileText(original, dir string) (string, error) {
	quoted := "'" + strings.ReplaceAll(dir, "'", `'"'"'`) + "'"
	block := beginMarker + "\ncase \":$PATH:\" in *:" + quoted + ":*) ;; *) PATH=\"${PATH:+$PATH:}\"" + quoted + "; export PATH ;; esac\n" + endMarker
	begin, end := strings.Count(original, beginMarker), strings.Count(original, endMarker)
	if begin == 0 && end == 0 {
		if original != "" && !strings.HasSuffix(original, "\n") {
			original += "\n"
		}
		return original + "\n" + block + "\n", nil
	}
	if begin != 1 || end != 1 {
		return "", fmt.Errorf("ambiguous ByeClaude PATH markers; existing shell settings were preserved")
	}
	start, finish := strings.Index(original, beginMarker), strings.Index(original, endMarker)
	if start > finish || (start > 0 && original[start-1] != '\n') || (finish > 0 && original[finish-1] != '\n') {
		return "", fmt.Errorf("invalid ByeClaude PATH block")
	}
	finish += len(endMarker)
	if finish < len(original) && original[finish] != '\n' {
		return "", fmt.Errorf("invalid ByeClaude PATH block ending")
	}
	return original[:start] + block + original[finish:], nil
}

func profileBytes(root *os.Root, name string) ([]byte, os.FileMode, error) {
	info, err := root.Lstat(name)
	if os.IsNotExist(err) {
		return nil, 0600, nil
	}
	if err != nil {
		return nil, 0, err
	}
	if !info.Mode().IsRegular() || info.Size() > 1024*1024 {
		return nil, 0, fmt.Errorf("shell profile is linked, non-regular or too large; edit its PATH setting manually")
	}
	f, err := root.Open(name)
	if err != nil {
		return nil, 0, err
	}
	defer f.Close()
	data, err := io.ReadAll(io.LimitReader(f, 1024*1024+1))
	if err != nil {
		return nil, 0, err
	}
	if len(data) > 1024*1024 {
		return nil, 0, fmt.Errorf("shell profile is too large")
	}
	return data, info.Mode().Perm(), nil
}

func updateProfile(path, dir string) error {
	root, err := os.OpenRoot(filepath.Dir(path))
	if err != nil {
		return err
	}
	defer root.Close()
	name := filepath.Base(path)
	original, mode, err := profileBytes(root, name)
	if err != nil {
		return err
	}
	updated, err := profileText(string(original), dir)
	if err != nil {
		return err
	}
	if updated == string(original) {
		return nil
	}
	var nonce [16]byte
	if _, err := rand.Read(nonce[:]); err != nil {
		return err
	}
	stage := ".byeclaude-path-" + hex.EncodeToString(nonce[:])
	f, err := root.OpenFile(stage, os.O_CREATE|os.O_EXCL|os.O_WRONLY, mode)
	if err != nil {
		return err
	}
	defer func() { _ = root.Remove(stage) }()
	_, writeErr := f.WriteString(updated)
	if writeErr == nil {
		writeErr = f.Sync()
	}
	closeErr := f.Close()
	if writeErr != nil {
		return writeErr
	}
	if closeErr != nil {
		return closeErr
	}
	current, _, err := profileBytes(root, name)
	if err != nil {
		return err
	}
	if !bytes.Equal(original, current) {
		return fmt.Errorf("shell profile changed during PATH setup; retry after your other editor finishes")
	}
	return root.Rename(stage, name)
}
