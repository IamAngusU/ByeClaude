package githubverify

import "testing"

func TestVerificationNeverClaimsAllCopiesGone(t *testing.T) {
	r := Report{Remote: RemoteHistory{Status: "clean"}, PullRefs: PullHistory{Status: "clean"}, Contributors: ContributorCheck{Status: "not_requested"}}
	if got := Classify(r); got != "clean_in_checked_scopes" {
		t.Fatalf("got %s", got)
	}
	r.PullRefs.Status = "partial"
	if got := Classify(r); got != "incomplete" {
		t.Fatalf("got %s", got)
	}
	r.PullRefs.Status = "matches_found"
	if got := Classify(r); got != "residual_evidence" {
		t.Fatalf("got %s", got)
	}
	r.PullRefs.Status = "clean"
	r.Contributors.Status = "listed"
	if got := Classify(r); got != "residual_evidence" {
		t.Fatalf("got %s", got)
	}
}
func TestGitHubUsernameValidation(t *testing.T) {
	for _, name := range []string{"../x", "some/name", "spaces here", "name?x"} {
		if ValidateGitHubUser(name) == nil {
			t.Errorf("accepted %q", name)
		}
	}
	if err := ValidateGitHubUser("claude"); err != nil {
		t.Fatal(err)
	}
}
