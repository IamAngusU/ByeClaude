package clean

import (
	"strings"
	"testing"

	"github.com/IamAngusU/ByeClaude/internal/preset"
)

func TestExplicitIdentityReplacementPreservesTimestampAndOtherFields(t *testing.T) {
	raw := "tree 0123456789012345678901234567890123456789\n" +
		"author Claude Bot <noreply@anthropic.com> 1234 +0530\n" +
		"committer A Human <human@example.org> 5678 -0400\n\n" +
		"Unrelated message\n"
	obj, err := parseCommit([]byte(raw))
	if err != nil {
		t.Fatal(err)
	}
	newObj, a, c, err := ReplaceMatchingCommitIdentities(obj, preset.Claude(), IdentityRewriteOptions{
		Author:    &IdentityReplacement{Name: "Correct Human", Email: "correct@example.org"},
		Committer: &IdentityReplacement{Name: "Correct Human", Email: "correct@example.org"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if a != 1 || c != 0 {
		t.Fatalf("rewrites: author=%d committer=%d", a, c)
	}
	reconstructed, dropped := rebuildCommit(newObj, nil, newObj.Message)
	if dropped != 0 {
		t.Fatalf("unexpected dropped headers: %d", dropped)
	}
	text := string(reconstructed)
	if !strings.Contains(text, "author Correct Human <correct@example.org> 1234 +0530") {
		t.Fatalf("missing rewritten header: %s", text)
	}
	if !strings.Contains(text, "committer A Human <human@example.org> 5678 -0400") {
		t.Fatalf("committer changed: %s", text)
	}
	if !strings.Contains(raw, "author Claude Bot") {
		t.Fatal("mutated original raw commit")
	}
}
func TestIdentityReplacementRejectsUnsafeInputs(t *testing.T) {
	for _, input := range []string{"", "Name", "Name <foo>", "Name <x@example.org>\nheader injected", "Name <>"} {
		if _, err := ParseIdentityReplacement(input); err == nil {
			t.Errorf("accepted %q", input)
		}
	}
	r, err := ParseIdentityReplacement("Human <human@example.org>")
	if err != nil || r.Name != "Human" {
		t.Fatalf("expected valid replacement: %v", err)
	}
	if err := (IdentityRewriteOptions{Author: &IdentityReplacement{Name: "Claude", Email: "noreply@anthropic.com"}}).Validate(preset.Claude()); err == nil {
		t.Fatal("replacement matching active rule must fail")
	}
}
func TestIdentityHeaderFailClosed(t *testing.T) {
	obj, err := parseCommit([]byte("tree a\nauthor malformed\ncommitter Me <me@example.org> 1 +0000\n\nx"))
	if err != nil {
		t.Fatal(err)
	}
	_, _, _, err = ReplaceMatchingCommitIdentities(obj, preset.Claude(), IdentityRewriteOptions{Author: &IdentityReplacement{Name: "Human", Email: "human@example.org"}})
	if err == nil {
		t.Fatal("expected malformed-header refusal")
	}
}
