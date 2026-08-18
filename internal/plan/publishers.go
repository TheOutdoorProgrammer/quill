package plan

import (
	"fmt"
	"sort"
	"strings"
)

// Publishers is what a release ships through. It is a set rather than a choice
// because a repo can carry both a Go binary and an iOS build, and both want the
// same version and the same tag.
type Publishers struct {
	GoReleaser bool
	Fledge     bool
}

// Any reports whether anything at all publishes. When nothing does, quill still
// tags and still writes a GitHub release, which is a useful release on its own.
func (p Publishers) Any() bool { return p.GoReleaser || p.Fledge }

func (p Publishers) String() string {
	var names []string
	if p.GoReleaser {
		names = append(names, "goreleaser")
	}
	if p.Fledge {
		names = append(names, "fledge")
	}
	if len(names) == 0 {
		return "none"
	}
	return strings.Join(names, ", ")
}

// ParsePublishers reads the input list. Separators are commas or any
// whitespace, so both a YAML one-liner and a block list work.
func ParsePublishers(s string) (Publishers, error) {
	var p Publishers
	var unknown []string
	sawNone := false

	for _, name := range strings.FieldsFunc(s, func(r rune) bool {
		return r == ',' || r == '\n' || r == '\r' || r == '\t' || r == ' '
	}) {
		switch strings.ToLower(name) {
		case "goreleaser":
			p.GoReleaser = true
		case "fledge":
			p.Fledge = true
		case "none":
			sawNone = true
		default:
			unknown = append(unknown, name)
		}
	}

	if len(unknown) > 0 {
		sort.Strings(unknown)
		return Publishers{}, fmt.Errorf("unknown publisher(s) %s, want goreleaser, fledge or none",
			strings.Join(unknown, ", "))
	}
	// Checked after the whole list, so ordering cannot hide the contradiction.
	if sawNone && p.Any() {
		return Publishers{}, fmt.Errorf("publish lists none alongside %s", p)
	}
	return p, nil
}
