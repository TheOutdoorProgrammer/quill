package main

import (
	"os"
	"path/filepath"
	"testing"
)

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
