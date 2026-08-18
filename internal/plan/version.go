// Package plan works out the version a release should carry, so that nobody
// reads the tag list and guesses.
package plan

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
)

// Version is a release-candidate-aware semver triple. Quill deliberately
// understands one prerelease shape and rejects every other, because a tag it
// half understands picks the wrong version for every release after it.
type Version struct {
	Major, Minor, Patch int
	RC                  int // 0 when this is not a candidate
}

func (v Version) String() string {
	s := fmt.Sprintf("v%d.%d.%d", v.Major, v.Minor, v.Patch)
	if v.RC > 0 {
		s += fmt.Sprintf("-rc.%d", v.RC)
	}
	return s
}

// Prerelease reports whether this version is a candidate, which is what
// decides GitHub's prerelease flag and whether the major alias moves.
func (v Version) Prerelease() bool { return v.RC > 0 }

// Release is v with any candidate suffix removed, so that a candidate and the
// release it precedes compare as the same line of work.
func (v Version) Release() Version { v.RC = 0; return v }

// MajorAlias is the moving tag people pin, as in actions/checkout@v4.
func (v Version) MajorAlias() string { return fmt.Sprintf("v%d", v.Major) }

// Less orders versions, with a candidate sorting BEFORE the release it
// precedes: v1.2.0-rc.1 comes before v1.2.0, which is what semver requires and
// what makes "the highest tag" mean the right thing.
func (v Version) Less(o Version) bool {
	switch {
	case v.Major != o.Major:
		return v.Major < o.Major
	case v.Minor != o.Minor:
		return v.Minor < o.Minor
	case v.Patch != o.Patch:
		return v.Patch < o.Patch
	case (v.RC == 0) != (o.RC == 0):
		return v.RC != 0 // a candidate precedes its release
	default:
		return v.RC < o.RC
	}
}

// Parse reads a tag. The bool reports whether it is a shape quill owns.
func Parse(tag string) (Version, bool) {
	var v Version
	s := strings.TrimPrefix(strings.TrimSpace(tag), "v")
	if s == "" {
		return v, false
	}
	if base, suffix, found := strings.Cut(s, "-rc."); found {
		n, err := strconv.Atoi(suffix)
		if err != nil || n < 1 {
			return v, false
		}
		v.RC, s = n, base
	} else if strings.Contains(s, "-") {
		return v, false // some other prerelease shape; not ours to reason about
	}

	parts := strings.Split(s, ".")
	if len(parts) != 3 {
		return v, false
	}
	fields := []*int{&v.Major, &v.Minor, &v.Patch}
	for i, p := range parts {
		n, err := strconv.Atoi(p)
		if err != nil || n < 0 {
			return v, false
		}
		*fields[i] = n
	}
	return v, true
}

// Latest parses tags, drops any it does not recognise, and returns the highest
// along with the full sorted set. Git prints tags in lexical order, which puts
// v1.9.0 after v1.10.0.
func Latest(raw []string) (Version, bool, []Version) {
	var tags []Version
	for _, r := range raw {
		if v, ok := Parse(r); ok {
			tags = append(tags, v)
		}
	}
	sort.Slice(tags, func(i, j int) bool { return tags[i].Less(tags[j]) })

	if len(tags) == 0 {
		return Version{}, false, nil
	}
	return tags[len(tags)-1], true, tags
}

// Bump is which part of the version to move.
type Bump string

const (
	Patch Bump = "patch"
	Minor Bump = "minor"
	Major Bump = "major"
)

// ParseBump rejects anything it does not know, because a typo that silently
// fell back to "patch" would ship the wrong version with no warning.
func ParseBump(s string) (Bump, error) {
	switch b := Bump(strings.ToLower(strings.TrimSpace(s))); b {
	case Patch, Minor, Major:
		return b, nil
	default:
		return "", fmt.Errorf("unknown bump %q, want patch, minor or major", s)
	}
}

// Next picks the version to cut. A candidate continues the line it is on: once
// v1.2.0-rc.1 exists the next is rc.2 for the SAME version, or the candidate
// races ahead of the release it exists to rehearse.
func Next(latest Version, haveTag bool, tags []Version, bump Bump, candidate bool) (Version, error) {
	if _, err := ParseBump(string(bump)); err != nil {
		return Version{}, err
	}

	// With no tags the bump applies to an implicit v0.0.0, so a project that
	// means to open at 1.0.0 can say so. Special casing the first release to
	// v0.1.0 would make that version unreachable without a throwaway tag.
	if !haveTag {
		var first Version
		switch bump {
		case Major:
			first = Version{Major: 1}
		case Minor:
			first = Version{Minor: 1}
		case Patch:
			first = Version{Patch: 1}
		}
		if candidate {
			first.RC = 1
		}
		return first, nil
	}

	if candidate && latest.RC > 0 {
		latest.RC++
		return latest, nil
	}

	// Promoting a candidate takes its version and ignores the bump: v1.3.0-rc.1
	// IS v1.3.0 in rehearsal, so bumping again would ship a release the
	// candidate never tested and strand v1.3.0 at a version that never happens.
	if !candidate && latest.RC > 0 {
		return latest.Release(), nil
	}

	base := latest.Release()
	switch bump {
	case Major:
		base = Version{Major: base.Major + 1}
	case Minor:
		base = Version{Major: base.Major, Minor: base.Minor + 1}
	case Patch:
		base.Patch++
	}

	if candidate {
		base.RC = highestRC(tags, base) + 1
	}
	return base, nil
}

// highestRC is the largest candidate number already cut for a version.
func highestRC(tags []Version, of Version) int {
	highest := 0
	for _, t := range tags {
		if t.RC > highest && t.Release() == of.Release() {
			highest = t.RC
		}
	}
	return highest
}
