package blacklist

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/IamAngusU/ByeClaude/internal/attribution"
	"github.com/IamAngusU/ByeClaude/internal/gitx"
)

func policyRepo(t *testing.T) *gitx.Repo {
	t.Helper()
	dir := t.TempDir()
	if out, err := exec.Command("git", "init", "-q", dir).CombinedOutput(); err != nil {
		t.Fatalf("init: %s %v", out, err)
	}
	repo, err := gitx.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	return repo
}

func TestPolicyPersistenceAndExplicitOverride(t *testing.T) {
	repo := policyRepo(t)
	set, saved, err := Load(repo)
	if err != nil || saved || !set.Match("Claude", "noreply@anthropic.com") {
		t.Fatalf("default: %+v %v", set, err)
	}
	if _, err := repo.Run("config", "--local", "user.name", "Preserved User"); err != nil {
		t.Fatal(err)
	}
	set.Rules = append(set.Rules, attribution.Rule{RuleID: "helper", ExactEmails: []string{"helper@example.org"}})
	if err := Save(repo, set, false); err != nil {
		t.Fatal(err)
	}
	matcher, err := Resolve(repo, "")
	if err != nil || !matcher.Match("Helper", "helper@example.org") {
		t.Fatal(err)
	}
	_, saved, err = Load(repo)
	if err != nil || !saved {
		t.Fatal("saved policy missing")
	}
	name, err := repo.Run("config", "--local", "user.name")
	if err != nil || string(name) != "Preserved User\n" {
		t.Fatal("unrelated config changed")
	}
	path := filepath.Join(t.TempDir(), "rules.json")
	if err := os.WriteFile(path, []byte(`{"rules":[{"id":"other","exact_emails":["other@example.org"]}]}`), 0600); err != nil {
		t.Fatal(err)
	}
	matcher, err = Resolve(repo, path)
	if err != nil || matcher.Match("Helper", "helper@example.org") || !matcher.Match("Other", "other@example.org") {
		t.Fatal("explicit policy did not override")
	}
}

func TestPolicyFailsClosedForCorruptionAndConflictingValues(t *testing.T) {
	repo := policyRepo(t)
	if _, err := repo.Run("config", "--local", configKey, "broken"); err != nil {
		t.Fatal(err)
	}
	if _, _, err := Load(repo); err == nil {
		t.Fatal("corrupt policy accepted")
	}
	if err := Save(repo, Default(), false); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.Run("config", "--local", "--add", configKey, `{"rules":[{"id":"other","exact_emails":["other@example.org"]}]}`); err != nil {
		t.Fatal(err)
	}
	if _, _, err := Load(repo); err == nil {
		t.Fatal("multiple config entries silently merged")
	}
}

func TestConfigLockAndInvalidEditsPreservePolicy(t *testing.T) {
	repo := policyRepo(t)
	if err := Save(repo, Default(), false); err != nil {
		t.Fatal(err)
	}
	if err := Save(repo, attribution.RuleSet{}, false); err == nil {
		t.Fatal("empty policy accepted")
	}
	lock := filepath.Join(repo.GitDir, "config.lock")
	if err := os.WriteFile(lock, []byte("other writer"), 0600); err != nil {
		t.Fatal(err)
	}
	set := attribution.RuleSet{Rules: []attribution.Rule{{RuleID: "other", ExactEmails: []string{"other@example.org"}}}}
	if err := Save(repo, set, false); err == nil {
		t.Fatal("concurrent Git config lock ignored")
	}
	current, saved, err := Load(repo)
	if err != nil || !saved || len(current.Rules) != 1 || current.Rules[0].RuleID != "claude-anthropic" {
		t.Fatal("failed save changed policy")
	}
}
