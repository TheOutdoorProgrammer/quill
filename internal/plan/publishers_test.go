package plan

import (
	"slices"
	"testing"
)

func TestParsePublishers(t *testing.T) {
	cases := []struct {
		in   string
		want Publishers
	}{
		{"", nil},
		{"none", nil},
		{"goreleaser", Publishers{GoReleaser}},
		{"docker", Publishers{Docker}},
		{"fledge", Publishers{Fledge}},

		{"goreleaser,fledge,docker", Publishers{GoReleaser, Fledge, Docker}},
		{"  DOCKER   GoReleaser  ", Publishers{GoReleaser, Docker}},
		{"docker\nfledge", Publishers{Fledge, Docker}},
		{"docker,docker", Publishers{Docker}},
	}

	for _, c := range cases {
		got, err := ParsePublishers(c.in)
		if err != nil {
			t.Errorf("ParsePublishers(%q): %v", c.in, err)
			continue
		}
		if !slices.Equal(got, c.want) {
			t.Errorf("ParsePublishers(%q) = %v, want %v", c.in, got, c.want)
		}
	}
}

// Listing them in a different order does not run them in a different order, so
// the parsed result reports what will happen rather than what was typed.
func TestParsePublishersIgnoresTheOrderTheyWereTypedIn(t *testing.T) {
	orderings := []string{
		"goreleaser,fledge,docker",
		"docker,fledge,goreleaser",
		"fledge,goreleaser,docker",
		"docker,goreleaser,fledge",
	}

	want := Publishers{GoReleaser, Fledge, Docker}
	for _, in := range orderings {
		got, err := ParsePublishers(in)
		if err != nil {
			t.Errorf("ParsePublishers(%q): %v", in, err)
			continue
		}
		if !slices.Equal(got, want) {
			t.Errorf("ParsePublishers(%q) = %v, want %v", in, got, want)
		}
		if got.String() != "goreleaser, fledge, docker" {
			t.Errorf("ParsePublishers(%q).String() = %q, want the run order", in, got.String())
		}
	}
}

func TestParsePublishersRejectsNonsense(t *testing.T) {
	bad := []string{
		"gorelease", // a typo would otherwise publish nothing, silently
		"dockerr",
		"homebrew",
		"goreleaser,none", // contradiction, either order
		"none,goreleaser",
		"none, fledge",
	}
	for _, in := range bad {
		if got, err := ParsePublishers(in); err == nil {
			t.Errorf("ParsePublishers(%q) = %v, want an error", in, got)
		}
	}
}

// Docker runs last because an image is the only artefact likely to consume
// something another publisher produced.
func TestOrderPutsDockerLastAndGoReleaserFirst(t *testing.T) {
	if Order[0] != GoReleaser {
		t.Errorf("Order starts with %q, want goreleaser", Order[0])
	}
	if Order[len(Order)-1] != Docker {
		t.Errorf("Order ends with %q, want docker", Order[len(Order)-1])
	}
	if len(Order) != 3 {
		t.Errorf("Order has %d publishers, want 3", len(Order))
	}
}

func TestHas(t *testing.T) {
	p := Publishers{Fledge, Docker}

	for _, want := range []Publisher{Fledge, Docker} {
		if !p.Has(want) {
			t.Errorf("Has(%q) = false, want true", want)
		}
	}
	if p.Has(GoReleaser) {
		t.Error("Has(goreleaser) = true, want false")
	}
	if (Publishers{}).Has(Docker) {
		t.Error("Has on nothing = true, want false")
	}
}

func TestPublishersString(t *testing.T) {
	cases := map[string]Publishers{
		"none":                       nil,
		"docker":                     {Docker},
		"goreleaser, docker":         {GoReleaser, Docker},
		"goreleaser, fledge, docker": {GoReleaser, Fledge, Docker},
	}
	for want, p := range cases {
		if got := p.String(); got != want {
			t.Errorf("%v.String() = %q, want %q", p, got, want)
		}
	}
}
