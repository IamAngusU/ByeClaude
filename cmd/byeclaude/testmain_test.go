package main

import (
	"os"
	"testing"
)

func TestMain(m *testing.M) {
	// Ordinary regression tests must never alter the developer's local counters.
	_ = os.Setenv("BYECLAUDE_METRICS", "off")
	os.Exit(m.Run())
}
