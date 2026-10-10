package clean

import (
	"encoding/json"
	"os"
	"testing"

	"github.com/IamAngusU/ByeClaude/internal/preset"
)

func TestClaudeMessageCorpus(t *testing.T) {
	raw, err := os.ReadFile("testdata/claude-message-corpus.json")
	if err != nil {
		t.Fatal(err)
	}
	var cases []struct {
		Name    string `json:"name"`
		Source  string `json:"source"`
		Message string `json:"message"`
		Removed int    `json:"removed"`
		Want    string `json:"want"`
	}
	if err := json.Unmarshal(raw, &cases); err != nil {
		t.Fatal(err)
	}
	for _, tc := range cases {
		t.Run(tc.Name, func(t *testing.T) {
			got, removed := StripMatchingTrailers(tc.Message, preset.Claude())
			if len(removed) != tc.Removed {
				t.Fatalf("removed %d entries, want %d; source %s; entries=%q", len(removed), tc.Removed, tc.Source, removed)
			}
			if got != tc.Want {
				t.Fatalf("cleaned message mismatch for %s\ngot:  %q\nwant: %q", tc.Source, got, tc.Want)
			}
		})
	}
}
