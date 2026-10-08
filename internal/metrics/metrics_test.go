package metrics

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestConcurrentCountersAndPrivacy(t *testing.T) {
	s := Store{Dir: t.TempDir(), LockTimeout: 5 * time.Second}
	var wg sync.WaitGroup
	for i := 0; i < 24; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := s.Add(Counters{Scans: 1, CommitsInspected: 7, CleanupCredits: 2}); err != nil {
				t.Error(err)
			}
		}()
	}
	wg.Wait()
	state, err := s.Read()
	if err != nil {
		t.Fatal(err)
	}
	if state.Counters.Scans != 24 || state.Counters.CommitsInspected != 168 || state.Counters.CleanupCredits != 48 {
		t.Fatal(state)
	}
	if state.Since == "" || !state.Enabled {
		t.Fatal(state)
	}
	data, err := os.ReadFile(filepath.Join(s.Dir, "metrics.json"))
	if err != nil {
		t.Fatal(err)
	}
	for _, private := range []string{s.Dir, "repository", "email", "commit_id", "command", "hostname"} {
		if bytes.Contains(data, []byte(private)) {
			t.Fatalf("unexpected personal field %q", private)
		}
	}
	entries, _ := os.ReadDir(s.Dir)
	if len(entries) != 2 {
		t.Fatalf("expected only state and permanent lock; got %d files", len(entries))
	}
}

func TestDisabledResetAndCorruptState(t *testing.T) {
	s := Store{Dir: t.TempDir()}
	if err := s.Enable(false); err != nil {
		t.Fatal(err)
	}
	if err := s.Add(Counters{HookCredits: 3}); !errors.Is(err, errDisabled) {
		t.Fatal(err)
	}
	if err := s.Reset(); err != nil {
		t.Fatal(err)
	}
	state, _ := s.Read()
	if state.Enabled || state.Counters.HookCredits != 0 {
		t.Fatal(state)
	}
	if err := s.Enable(true); err != nil {
		t.Fatal(err)
	}
	if err := s.Add(Counters{HookEdits: 1, HookCredits: 3}); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(s.Dir, "metrics.json")
	for _, bad := range []string{`{broken`, `{"schema":2,"enabled":true}`, `{"schema":1,"enabled":true,"unexpected":"x"}`, strings.Repeat("x", maxStateBytes+1), `{"schema":1,"enabled":true} {}`} {
		if err := os.WriteFile(path, []byte(bad), 0600); err != nil {
			t.Fatal(err)
		}
		if err := s.Add(Counters{Scans: 1}); err == nil {
			t.Fatal("corrupt state accepted")
		}
		after, _ := os.ReadFile(path)
		if string(after) != bad {
			t.Fatal("damaged data overwritten implicitly")
		}
	}
	if err := s.Reset(); err != nil {
		t.Fatal(err)
	}
	state, err := s.Read()
	if err != nil || state.Enabled || state.Counters != (Counters{}) {
		t.Fatal(state, err)
	}
}

func TestMetricsUnavailableNeverEscapesRecord(t *testing.T) {
	base := t.TempDir()
	dir := filepath.Join(base, "not-a-directory")
	if err := os.WriteFile(dir, []byte("keep"), 0600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("BYECLAUDE_STATE_DIR", dir)
	t.Setenv("BYECLAUDE_METRICS", "on")
	Record(Counters{Scans: 1})
	if got, _ := os.ReadFile(dir); string(got) != "keep" {
		t.Fatal("unavailable storage overwritten")
	}
	t.Setenv("BYECLAUDE_STATE_DIR", filepath.Join(base, "unused"))
	t.Setenv("BYECLAUDE_METRICS", "off")
	Record(Counters{Scans: 1})
	if _, err := os.Stat(filepath.Join(base, "unused")); !os.IsNotExist(err) {
		t.Fatal("disabled collection created storage")
	}
	for _, value := range []string{"true", "1", "on"} {
		t.Setenv("BYECLAUDE_METRICS", value)
		if SessionPaused() {
			t.Fatal(value)
		}
	}
	for _, value := range []string{"false", "0", "off", "garbage"} {
		t.Setenv("BYECLAUDE_METRICS", value)
		if !SessionPaused() {
			t.Fatal(value)
		}
	}
	t.Setenv("BYECLAUDE_METRICS", "")
	t.Setenv("CI", "true")
	if !SessionPaused() {
		t.Fatal("CI must pause collection")
	}
	t.Setenv("CI", "false")
	if SessionPaused() {
		t.Fatal("false CI flag")
	}
}

func TestCounterOverflowDoesNotReplacePreviousState(t *testing.T) {
	s := Store{Dir: t.TempDir()}
	if err := s.Add(Counters{Scans: math.MaxUint64}); err != nil {
		t.Fatal(err)
	}
	if err := s.Add(Counters{Scans: 1}); err == nil {
		t.Fatal("overflow accepted")
	}
	state, _ := s.Read()
	if state.Counters.Scans != math.MaxUint64 {
		t.Fatal(state)
	}
}

func TestBusyStoreIsBounded(t *testing.T) {
	s := Store{Dir: t.TempDir(), LockTimeout: 20 * time.Millisecond}
	unlock, err := tryLock(filepath.Join(s.Dir, "metrics.lock"))
	if err != nil {
		t.Fatal(err)
	}
	defer unlock()
	started := time.Now()
	err = s.Add(Counters{Scans: 1})
	if !errors.Is(err, errBusy) || time.Since(started) > time.Second {
		t.Fatal(err)
	}
}

func TestLockChild(t *testing.T) {
	path := os.Getenv("BYECLAUDE_METRICS_TEST_LOCK")
	if path == "" {
		return
	}
	unlock, err := tryLock(path)
	if err != nil {
		t.Fatal(err)
	}
	defer unlock()
	fmt.Println("locked")
	time.Sleep(30 * time.Second)
}

func TestLockIsReleasedAfterProcessDies(t *testing.T) {
	path := filepath.Join(t.TempDir(), "metrics.lock")
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(exe, "-test.run=^TestLockChild$")
	cmd.Env = append(os.Environ(), "BYECLAUDE_METRICS_TEST_LOCK="+path)
	pipe, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = cmd.Process.Kill(); _ = cmd.Wait() }()
	ready, err := bufio.NewReader(pipe).ReadString('\n')
	if err != nil || strings.TrimSpace(ready) != "locked" {
		t.Fatal(ready, err)
	}
	if unlock, err := tryLock(path); !errors.Is(err, errBusy) {
		if unlock != nil {
			unlock()
		}
		t.Fatalf("second process acquired lock: %v", err)
	}
	if err := cmd.Process.Kill(); err != nil {
		t.Fatal(err)
	}
	_ = cmd.Wait()
	unlock, err := tryLock(path)
	if err != nil {
		t.Fatalf("stale lock after process death: %v", err)
	}
	unlock()
}
