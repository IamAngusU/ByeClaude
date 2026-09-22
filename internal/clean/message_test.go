package clean

import "testing"

func TestStripClaudeTrailers(t *testing.T) {
	in := "fix: thing\n\nCo-Authored-By: Claude Opus 4.8 <noreply@anthropic.com>\nCo-authored-by: Human <human@example.com>\n"
	got, removed := StripClaudeTrailers(in)
	want := "fix: thing\n\nCo-authored-by: Human <human@example.com>\n"
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
	if len(removed) != 1 {
		t.Fatalf("removed=%d", len(removed))
	}
}

func TestDoesNotRemoveHumanClaude(t *testing.T) {
	in := "feat\n\nCo-authored-by: Claude Shannon <shannon@example.org>\n"
	got, removed := StripClaudeTrailers(in)
	if got != in || len(removed) != 0 {
		t.Fatalf("unexpected removal: %q %#v", got, removed)
	}
}

func TestDoesNotRemoveBodyExample(t *testing.T) {
	in := "docs: explain format\n\nExample output:\nCo-Authored-By: Claude Opus 4.8 <noreply@anthropic.com>\nThis line makes it body text.\n"
	got, removed := StripClaudeTrailers(in)
	if got != in || len(removed) != 0 {
		t.Fatalf("body example was changed: %q %#v", got, removed)
	}
}

func TestOnlyClaudeRemovedFromTrailerBlock(t *testing.T) {
	in := "feat\n\nSigned-off-by: Angus <angus@example.com>\nCo-Authored-By: Claude <noreply@anthropic.com>\n"
	got, removed := StripClaudeTrailers(in)
	want := "feat\n\nSigned-off-by: Angus <angus@example.com>\n"
	if got != want || len(removed) != 1 {
		t.Fatalf("got %q removed=%#v", got, removed)
	}
}

func TestClaudeTrailersIgnoresBodyExample(t *testing.T) {
	in := "docs: explain format\n\nExample output:\nCo-Authored-By: Claude Opus 4.8 <noreply@anthropic.com>\nThis line makes it body text.\n"
	if got := ClaudeTrailers(in); len(got) != 0 {
		t.Fatalf("body example reported as trailer: %#v", got)
	}
}

func TestClaudeTrailersFindsOnlyFinalTrailerBlock(t *testing.T) {
	in := "feat\n\nClaude may appear in prose.\n\nSigned-off-by: Angus <angus@example.com>\nCo-Authored-By: Claude <noreply@anthropic.com>\n"
	got := ClaudeTrailers(in)
	if len(got) != 1 || got[0] != "Co-Authored-By: Claude <noreply@anthropic.com>" {
		t.Fatalf("got %#v", got)
	}
}

func TestStripTagSignatureFormats(t *testing.T) {
	cases := []string{
		"-----BEGIN PGP SIGNATURE-----",
		"-----BEGIN PGP MESSAGE-----",
		"-----BEGIN SSH SIGNATURE-----",
		"-----BEGIN SIGNED MESSAGE-----",
	}
	for _, marker := range cases {
		t.Run(marker, func(t *testing.T) {
			msg := "release tag\n" + marker + "\nopaque\n"
			got, dropped := stripTagSignature(msg)
			if !dropped || got != "release tag\n" {
				t.Fatalf("got %q dropped=%v", got, dropped)
			}
		})
	}
}
