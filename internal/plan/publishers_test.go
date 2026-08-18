package plan

import "testing"

func TestParsePublishers(t *testing.T) {
	cases := []struct {
		in         string
		goreleaser bool
		fledge     bool
	}{
		{"", false, false},
		{"none", false, false},
		{"goreleaser", true, false},
		{"fledge", false, true},

		// A repo shipping a binary and an iOS build wants one tag for both.
		{"goreleaser,fledge", true, true},
		{"goreleaser, fledge", true, true},
		{"goreleaser\nfledge", true, true},
		{"  GoReleaser   FLEDGE  ", true, true},

		{"fledge,fledge", false, true},
	}

	for _, c := range cases {
		got, err := ParsePublishers(c.in)
		if err != nil {
			t.Errorf("ParsePublishers(%q): %v", c.in, err)
			continue
		}
		if got.GoReleaser != c.goreleaser || got.Fledge != c.fledge {
			t.Errorf("ParsePublishers(%q) = %+v, want goreleaser=%v fledge=%v",
				c.in, got, c.goreleaser, c.fledge)
		}
	}
}

func TestParsePublishersRejectsNonsense(t *testing.T) {
	bad := []string{
		"gorelease", // a typo would otherwise publish nothing, silently
		"homebrew",
		"goreleaser,none", // contradiction, either order
		"none,goreleaser",
		"none, fledge",
	}
	for _, in := range bad {
		if got, err := ParsePublishers(in); err == nil {
			t.Errorf("ParsePublishers(%q) = %+v, want an error", in, got)
		}
	}
}

func TestPublishersString(t *testing.T) {
	cases := map[string]Publishers{
		"none":               {},
		"goreleaser":         {GoReleaser: true},
		"fledge":             {Fledge: true},
		"goreleaser, fledge": {GoReleaser: true, Fledge: true},
	}
	for want, p := range cases {
		if got := p.String(); got != want {
			t.Errorf("%+v.String() = %q, want %q", p, got, want)
		}
	}
}
