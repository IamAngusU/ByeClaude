// Package metrics stores optional local counters. It never makes network calls
// and accepts no repository paths, identities, commit IDs or command arguments.
package metrics

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/IamAngusU/ByeClaude/internal/userstate"
)

const maxStateBytes = 16384

type Counters struct {
	Scans            uint64 `json:"scans"`
	Checks           uint64 `json:"checks"`
	CommitsInspected uint64 `json:"commits_inspected"`
	Cleanups         uint64 `json:"cleanups"`
	CleanupCredits   uint64 `json:"cleanup_credits_removed"`
	CommitsRewritten uint64 `json:"commits_rewritten"`
	IdentityFields   uint64 `json:"identity_fields_corrected"`
	HookEdits        uint64 `json:"commit_messages_cleaned"`
	HookCredits      uint64 `json:"hook_credits_removed"`
	PushChecks       uint64 `json:"push_checks"`
	PushBlocks       uint64 `json:"push_attempts_blocked"`
	WorkMS           uint64 `json:"recorded_work_ms"`
}

type State struct {
	Schema   int      `json:"schema"`
	Enabled  bool     `json:"enabled"`
	Since    string   `json:"since,omitempty"`
	Counters Counters `json:"counters"`
}

type Store struct {
	Dir         string
	LockTimeout time.Duration
}

func DefaultStore() (Store, error) {
	dir, err := userstate.Directory()
	return Store{Dir: dir}, err
}

func SessionPaused() bool {
	switch strings.ToLower(strings.TrimSpace(os.Getenv("BYECLAUDE_METRICS"))) {
	case "on", "1", "true":
		return false
	case "":
		ci := strings.ToLower(strings.TrimSpace(os.Getenv("CI")))
		return ci != "" && ci != "0" && ci != "false"
	default:
		return true // Unknown overrides must not accidentally enable collection.
	}
}

// Record is best-effort: statistics must never fail a Git operation or a hook.
func Record(delta Counters) {
	if SessionPaused() {
		return
	}
	store, err := DefaultStore()
	if err == nil {
		_ = store.Add(delta)
	}
}

func (s Store) Read() (State, error) {
	root, err := os.OpenRoot(s.Dir)
	if os.IsNotExist(err) {
		return State{Schema: 1, Enabled: true}, nil
	}
	if err != nil {
		return State{}, err
	}
	defer root.Close()
	return read(root)
}

func read(root *os.Root) (State, error) {
	state := State{Schema: 1, Enabled: true}
	info, err := root.Lstat("metrics.json")
	if os.IsNotExist(err) {
		return state, nil
	}
	if err != nil {
		return State{}, err
	}
	if !info.Mode().IsRegular() || info.Size() > maxStateBytes {
		return State{}, fmt.Errorf("metrics state must be a small regular file")
	}
	f, err := root.Open("metrics.json")
	if err != nil {
		return State{}, err
	}
	defer f.Close()
	data, err := io.ReadAll(io.LimitReader(f, maxStateBytes+1))
	if err != nil {
		return State{}, err
	}
	if len(data) > maxStateBytes {
		return State{}, fmt.Errorf("metrics state is too large")
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	// A missing enabled field must not turn collection on after corruption.
	state = State{}
	if err := decoder.Decode(&state); err != nil {
		return State{}, err
	}
	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		return State{}, fmt.Errorf("extra metrics data")
	}
	if state.Schema != 1 {
		return State{}, fmt.Errorf("unsupported metrics schema")
	}
	if state.Since != "" {
		if _, err := time.Parse(time.RFC3339, state.Since); err != nil {
			return State{}, err
		}
	}
	return state, nil
}

func (s Store) transaction(update func(*State) error, reset bool) error {
	if !filepath.IsAbs(s.Dir) {
		return fmt.Errorf("metrics directory must be absolute")
	}
	if err := os.MkdirAll(s.Dir, 0700); err != nil {
		return err
	}
	timeout := s.LockTimeout
	if timeout == 0 {
		timeout = 500 * time.Millisecond
	}
	deadline := time.Now().Add(timeout)
	var unlock func()
	var err error
	for {
		unlock, err = tryLock(filepath.Join(s.Dir, "metrics.lock"))
		if err == nil {
			break
		}
		if !errors.Is(err, errBusy) || time.Now().After(deadline) {
			return err
		}
		time.Sleep(5 * time.Millisecond)
	}
	defer unlock()
	root, err := os.OpenRoot(s.Dir)
	if err != nil {
		return err
	}
	defer root.Close()
	state, err := read(root)
	if err != nil {
		if !reset {
			return err
		}
		// Explicit reset repairs damaged counters but leaves collection disabled.
		state = State{Schema: 1, Enabled: false}
	}
	if err := update(&state); err != nil {
		return err
	}
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}
	var nonce [16]byte
	if _, err := rand.Read(nonce[:]); err != nil {
		return err
	}
	name := ".metrics-" + hex.EncodeToString(nonce[:])
	f, err := root.OpenFile(name, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		return err
	}
	defer func() { _ = root.Remove(name) }()
	_, writeErr := f.Write(append(data, '\n'))
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
	return root.Rename(name, "metrics.json")
}

var errBusy = errors.New("metrics are busy; try again")

func (s Store) Add(delta Counters) error {
	return s.transaction(func(state *State) error {
		if !state.Enabled {
			return errDisabled
		}
		pairs := [][2]*uint64{
			{&state.Counters.Scans, &delta.Scans}, {&state.Counters.Checks, &delta.Checks},
			{&state.Counters.CommitsInspected, &delta.CommitsInspected}, {&state.Counters.Cleanups, &delta.Cleanups},
			{&state.Counters.CleanupCredits, &delta.CleanupCredits}, {&state.Counters.CommitsRewritten, &delta.CommitsRewritten},
			{&state.Counters.IdentityFields, &delta.IdentityFields}, {&state.Counters.HookEdits, &delta.HookEdits},
			{&state.Counters.HookCredits, &delta.HookCredits}, {&state.Counters.PushChecks, &delta.PushChecks},
			{&state.Counters.PushBlocks, &delta.PushBlocks}, {&state.Counters.WorkMS, &delta.WorkMS},
		}
		for _, pair := range pairs {
			if *pair[0] > math.MaxUint64-*pair[1] {
				return fmt.Errorf("metrics counter overflow")
			}
			*pair[0] += *pair[1]
		}
		if state.Since == "" {
			state.Since = time.Now().UTC().Format(time.RFC3339)
		}
		return nil
	}, false)
}

var errDisabled = errors.New("local metrics are disabled")

func (s Store) Enable(enabled bool) error {
	return s.transaction(func(state *State) error { state.Enabled = enabled; return nil }, false)
}

func (s Store) Reset() error {
	return s.transaction(func(state *State) error {
		state.Counters = Counters{}
		state.Since = ""
		return nil
	}, true)
}
