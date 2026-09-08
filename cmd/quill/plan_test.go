package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestPlanPinsCheckoutBeforePublishing(t *testing.T) {
	dir := t.TempDir()
	git := func(args ...string) string {
		t.Helper()
		cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git: %v: %s", err, out)
		}
		return strings.TrimSpace(string(out))
	}
	git("init")
	git("-c", "user.name=Test", "-c", "user.email=test@example.com", "commit", "--allow-empty", "-m", "initial")
	head := git("rev-parse", "HEAD")
	t.Setenv("GITHUB_OUTPUT", filepath.Join(t.TempDir(), "outputs"))
	t.Setenv("GITHUB_STEP_SUMMARY", filepath.Join(t.TempDir(), "summary"))
	for _, source := range []string{"main", head[:12], strings.Repeat("0", 40), strings.ToUpper(head)} {
		if err := runPlan([]string{"-dir", dir, "-source-sha", source}); err == nil {
			t.Fatalf("accepted source %q", source)
		}
	}
	for _, source := range []string{"", head} {
		if err := runPlan([]string{"-dir", dir, "-source-sha", source}); err != nil {
			t.Fatal(err)
		}
	}
	if tags := git("tag", "--list"); tags != "" {
		t.Fatalf("planning created tags: %s", tags)
	}
}

func TestParseScope(t *testing.T) {
	candidates := []string{"Release Candidate", "release candidate", "rc", "CANDIDATE"}
	for _, in := range candidates {
		got, err := parseScope(in)
		if err != nil {
			t.Errorf("parseScope(%q): %v", in, err)
			continue
		}
		if !got {
			t.Errorf("parseScope(%q) = false, want a candidate", in)
		}
	}

	for _, in := range []string{"Release", "release", " RELEASE "} {
		got, err := parseScope(in)
		if err != nil {
			t.Errorf("parseScope(%q): %v", in, err)
			continue
		}
		if got {
			t.Errorf("parseScope(%q) = true, want a full release", in)
		}
	}

	for _, in := range []string{"", "beta", "Release Cand"} {
		if _, err := parseScope(in); err == nil {
			t.Errorf("parseScope(%q): accepted, want an error", in)
		}
	}
}

func TestGuardRef(t *testing.T) {
	if err := guardRef("refs/heads/topic", ""); err != nil {
		t.Errorf("no requirement should permit any ref: %v", err)
	}
	if err := guardRef("refs/heads/main", "refs/heads/main"); err != nil {
		t.Errorf("matching refs should pass: %v", err)
	}
	if err := guardRef("refs/heads/topic", "refs/heads/main"); err == nil {
		t.Error("releasing off a topic branch should be refused")
	}
	// A requirement that cannot be checked is a broken guard, not a pass.
	if err := guardRef("", "refs/heads/main"); err == nil {
		t.Error("an empty ref against a requirement should be refused")
	}
}

func TestCheckArtifacts(t *testing.T) {
	dir := t.TempDir()
	write := func(name string) string {
		t.Helper()
		path := filepath.Join(dir, name)
		if err := os.WriteFile(path, []byte("x"), 0o644); err != nil {
			t.Fatalf("writing %s: %v", name, err)
		}
		return path
	}

	one := write("App.ipa")
	if err := checkArtifacts([]string{one}); err != nil {
		t.Errorf("exactly one match should pass: %v", err)
	}
	if err := checkArtifacts(nil); err != nil {
		t.Errorf("no requirement should pass: %v", err)
	}

	if err := checkArtifacts([]string{filepath.Join(dir, "missing.ipa")}); err == nil {
		t.Error("a pattern matching nothing should be refused")
	}

	// Two matches is the dangerous case: a publisher would pick one silently.
	write("Other.ipa")
	if err := checkArtifacts([]string{filepath.Join(dir, "*.ipa")}); err == nil {
		t.Error("a pattern matching two files should be refused")
	}
}
