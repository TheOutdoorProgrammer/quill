package plan

import (
	"fmt"
	"sort"
	"strings"
)

// Publisher is one backend a release ships through.
type Publisher string

const (
	GoReleaser Publisher = "goreleaser"
	Fledge     Publisher = "fledge"
	Docker     Publisher = "docker"
)

// Order is the fixed sequence publishers run in (adr/0005). GoReleaser first
// because it produces the release the others might reference, Docker last
// because an image is the only artefact likely to consume one.
var Order = []Publisher{GoReleaser, Fledge, Docker}

// rank is a publisher's position in Order, so a parsed list can be sorted into
// the sequence it will actually run in rather than the one it was typed in.
var rank = func() map[Publisher]int {
	r := make(map[Publisher]int, len(Order))
	for i, p := range Order {
		r[p] = i
	}
	return r
}()

// Publishers is what a release ships through. It is a set: listing them in a
// different order does not run them in a different order.
type Publishers []Publisher

// Any reports whether anything at all publishes. When nothing does, quill still
// tags and still writes a GitHub release, which is a useful release on its own.
func (p Publishers) Any() bool { return len(p) > 0 }

// Has reports whether a publisher runs.
func (p Publishers) Has(want Publisher) bool {
	for _, got := range p {
		if got == want {
			return true
		}
	}
	return false
}

func (p Publishers) String() string {
	if len(p) == 0 {
		return "none"
	}
	names := make([]string, len(p))
	for i, publisher := range p {
		names[i] = string(publisher)
	}
	return strings.Join(names, ", ")
}

// ParsePublishers reads the input list, separated by commas or any whitespace.
// The result is sorted into Order, so String reports what will happen rather
// than what was asked for.
func ParsePublishers(s string) (Publishers, error) {
	var parsed Publishers
	var unknown []string
	seen := map[Publisher]bool{}
	sawNone := false

	for _, name := range strings.FieldsFunc(s, func(r rune) bool {
		return r == ',' || r == '\n' || r == '\r' || r == '\t' || r == ' '
	}) {
		lowered := strings.ToLower(name)
		if lowered == "none" {
			sawNone = true
			continue
		}

		publisher := Publisher(lowered)
		if _, ok := rank[publisher]; !ok {
			unknown = append(unknown, name)
			continue
		}
		if !seen[publisher] {
			seen[publisher] = true
			parsed = append(parsed, publisher)
		}
	}

	if len(unknown) > 0 {
		sort.Strings(unknown)
		return nil, fmt.Errorf("unknown publisher(s) %s, want %s or none",
			strings.Join(unknown, ", "), knownNames())
	}
	// Checked after the whole list, so ordering cannot hide the contradiction.
	if sawNone && parsed.Any() {
		return nil, fmt.Errorf("publish lists none alongside %s", parsed)
	}

	sort.Slice(parsed, func(i, j int) bool { return rank[parsed[i]] < rank[parsed[j]] })
	return parsed, nil
}

func knownNames() string {
	names := make([]string, 0, len(Order))
	for _, p := range Order {
		names = append(names, string(p))
	}
	sort.Strings(names)
	return strings.Join(names, ", ")
}
