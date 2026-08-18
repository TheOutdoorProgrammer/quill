package plan

import "testing"

func TestNext(t *testing.T) {
	cases := []struct {
		name      string
		tags      []string
		bump      Bump
		candidate bool
		want      string
	}{
		// No tags means the bump applies to an implicit v0.0.0, which is the
		// only way to open a project at 1.0.0 without a throwaway tag.
		{"first release", nil, Patch, false, "v0.0.1"},
		{"first minor", nil, Minor, false, "v0.1.0"},
		{"first major", nil, Major, false, "v1.0.0"},
		{"first candidate", nil, Patch, true, "v0.0.1-rc.1"},
		{"first major candidate", nil, Major, true, "v1.0.0-rc.1"},
		{"only a major alias still counts as no tags", []string{"v1"}, Major, false, "v1.0.0"},

		{"patch", []string{"v1.1.0", "v1.2.0"}, Patch, false, "v1.2.1"},
		{"minor", []string{"v1.1.0", "v1.2.0"}, Minor, false, "v1.3.0"},
		{"major", []string{"v1.1.0", "v1.2.0"}, Major, false, "v2.0.0"},

		{"major resets minor and patch", []string{"v1.4.7"}, Major, false, "v2.0.0"},
		{"minor resets patch", []string{"v1.4.7"}, Minor, false, "v1.5.0"},

		{"candidate for a patch", []string{"v1.2.0"}, Patch, true, "v1.2.1-rc.1"},
		{"candidate for a minor", []string{"v1.2.0"}, Minor, true, "v1.3.0-rc.1"},
		{"candidate continues its line", []string{"v1.2.0", "v1.3.0-rc.1"}, Patch, true, "v1.3.0-rc.2"},
		{"candidate counts to the highest", []string{"v1.3.0-rc.1", "v1.3.0-rc.2"}, Patch, true, "v1.3.0-rc.3"},

		// Promotion ignores the bump, or the candidate is stranded at a version
		// that never ships.
		{"promote a candidate", []string{"v1.2.0", "v1.3.0-rc.1"}, Patch, false, "v1.3.0"},
		{"promote, bump says minor", []string{"v1.2.0", "v1.3.0-rc.1"}, Minor, false, "v1.3.0"},
		{"promote, bump says major", []string{"v1.2.0", "v1.3.0-rc.1"}, Major, false, "v1.3.0"},
		{"promote the last candidate", []string{"v1.3.0-rc.1", "v1.3.0-rc.2"}, Patch, false, "v1.3.0"},

		// Tags arrive in whatever order git prints them, which is lexical.
		{"semver order, not lexical", []string{"v1.10.0", "v1.9.0"}, Patch, false, "v1.10.1"},
		{"rc sorts before its release", []string{"v1.3.0", "v1.3.0-rc.1"}, Patch, false, "v1.3.1"},
		{"unsorted input", []string{"v2.0.0", "v1.0.0", "v1.5.0"}, Patch, false, "v2.0.1"},

		// A moving major alias lives alongside the real tags and must never be
		// mistaken for one, or every release after v1 recomputes from "v1".
		{"major alias is not a version", []string{"v1", "v1.2.0"}, Patch, false, "v1.2.1"},
		{"only a major alias reads as no tags", []string{"v1"}, Patch, false, "v0.0.1"},

		// Foreign prerelease shapes are dropped rather than guessed at.
		{"foreign prerelease ignored", []string{"v1.2.0", "v1.3.0-beta.1"}, Patch, false, "v1.2.1"},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			latest, have, parsed := Latest(c.tags)
			got, err := Next(latest, have, parsed, c.bump, c.candidate)
			if err != nil {
				t.Fatalf("Next: %v", err)
			}
			if got.String() != c.want {
				t.Errorf("tags %v bump %q candidate %v: got %s, want %s",
					c.tags, c.bump, c.candidate, got, c.want)
			}
		})
	}
}

func TestNextRejectsUnknownBump(t *testing.T) {
	if _, err := Next(Version{}, true, nil, "sideways", false); err == nil {
		t.Fatal("expected an error for an unknown bump")
	}
}

func TestParseBump(t *testing.T) {
	for _, in := range []string{"patch", "Minor", " MAJOR "} {
		if _, err := ParseBump(in); err != nil {
			t.Errorf("ParseBump(%q): %v", in, err)
		}
	}
	for _, in := range []string{"", "pat", "bugfix", "release"} {
		if _, err := ParseBump(in); err == nil {
			t.Errorf("ParseBump(%q): accepted, want an error", in)
		}
	}
}

func TestParse(t *testing.T) {
	good := map[string]Version{
		"v1.2.3":         {Major: 1, Minor: 2, Patch: 3},
		"1.2.3":          {Major: 1, Minor: 2, Patch: 3},
		" v1.2.3 ":       {Major: 1, Minor: 2, Patch: 3},
		"v1.2.3-rc.4":    {Major: 1, Minor: 2, Patch: 3, RC: 4},
		"v0.0.0":         {},
		"v10.20.30":      {Major: 10, Minor: 20, Patch: 30},
		"v1.2.3-rc.1000": {Major: 1, Minor: 2, Patch: 3, RC: 1000},
	}
	for tag, want := range good {
		got, ok := Parse(tag)
		if !ok {
			t.Errorf("Parse(%q): rejected, want %v", tag, want)
			continue
		}
		if got != want {
			t.Errorf("Parse(%q) = %v, want %v", tag, got, want)
		}
	}

	// A tag that does not parse must be dropped rather than guessed at: one
	// misread tag picks the wrong version for every release after it.
	bad := []string{
		"", "v", "nightly", "v1.2", "v1.2.3.4", "v1.2.x",
		"v1.2.3-rc.0", "v1.2.3-rc.x", "v1.2.3-rc.", "v1.2.3-beta.1",
		"v-1.2.3", "v1.-2.3", "v1", "v2",
	}
	for _, tag := range bad {
		if got, ok := Parse(tag); ok {
			t.Errorf("Parse(%q) = %v, want rejected", tag, got)
		}
	}
}

func TestLessOrdersCandidateBeforeItsRelease(t *testing.T) {
	rc, _ := Parse("v1.3.0-rc.1")
	final, _ := Parse("v1.3.0")
	if !rc.Less(final) {
		t.Error("v1.3.0-rc.1 must sort before v1.3.0")
	}
	if final.Less(rc) {
		t.Error("v1.3.0 must not sort before v1.3.0-rc.1")
	}
}

func TestMajorAliasAndPrerelease(t *testing.T) {
	rc, _ := Parse("v2.1.0-rc.3")
	if !rc.Prerelease() {
		t.Error("a candidate must report as a prerelease")
	}
	if got := rc.MajorAlias(); got != "v2" {
		t.Errorf("MajorAlias = %q, want v2", got)
	}

	final, _ := Parse("v2.1.0")
	if final.Prerelease() {
		t.Error("a release must not report as a prerelease")
	}
}
