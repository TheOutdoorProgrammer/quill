package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/TheOutdoorProgrammer/quill/internal/actions"
	"github.com/TheOutdoorProgrammer/quill/internal/gitrepo"
	"github.com/TheOutdoorProgrammer/quill/internal/plan"
)

// artifactList collects a repeatable flag, so a repo publishing more than one
// thing can pre-flight all of them.
type artifactList []string

func (a *artifactList) String() string { return strings.Join(*a, ", ") }

func (a *artifactList) Set(v string) error {
	if v = strings.TrimSpace(v); v != "" {
		*a = append(*a, v)
	}
	return nil
}

func runPlan(args []string) error {
	fs := flag.NewFlagSet("plan", flag.ExitOnError)
	dir := fs.String("dir", ".", "the working tree to read")
	remote := fs.String("remote", "origin", "the remote to release to")
	bumpFlag := fs.String("bump", "patch", "which part to bump: major, minor or patch")
	scope := fs.String("scope", "Release", `"Release" or "Release Candidate"`)
	ref := fs.String("ref", "", "the ref this run is on, normally github.ref")
	requireRef := fs.String("require-ref", "", "fail unless -ref matches this")
	publish := fs.String("publish", "none",
		"publishers to run, in order: goreleaser, docker, fledge, or none")
	fledgeArtifact := fs.String("fledge-artifact", "",
		"glob for the archive fledge will publish, checked before tagging")
	dockerImages := fs.String("docker-images", "", "image name, defaulting to the repository")
	dockerRegistry := fs.String("docker-registry", "", "registry for the default image name")
	dockerTags := fs.String("docker-tags", "", "metadata-action tag spec, defaulting to semver")

	var artifacts artifactList
	fs.Var(&artifacts, "require-artifact", "glob that must match exactly one file; repeatable")

	if err := fs.Parse(args); err != nil {
		return err
	}

	bump, err := plan.ParseBump(*bumpFlag)
	if err != nil {
		return err
	}
	candidate, err := parseScope(*scope)
	if err != nil {
		return err
	}
	publishers, err := plan.ParsePublishers(*publish)
	if err != nil {
		return err
	}

	// An empty glob is not an omission to complain about: fledge can read the
	// path from its own fledge.yaml, in which case quill never sees one.
	if publishers.Has(plan.Fledge) && *fledgeArtifact != "" {
		artifacts = append(artifacts, *fledgeArtifact)
	}

	if err := guardRef(*ref, *requireRef); err != nil {
		return err
	}
	if err := checkArtifacts(artifacts); err != nil {
		return err
	}

	repo := gitrepo.New(*dir, *remote)

	// A shallow checkout has no tags, so the next version would restart at
	// v0.1.0 and quietly clobber the real line. Callers forget fetch-depth: 0.
	shallow, err := repo.Shallow()
	if err != nil {
		return err
	}
	if shallow {
		return fmt.Errorf("this checkout is shallow, so the tag list is missing. " +
			"Set `fetch-depth: 0` on actions/checkout")
	}

	tags, err := repo.Tags()
	if err != nil {
		return err
	}

	latest, haveTag, parsed := plan.Latest(tags)
	next, err := plan.Next(latest, haveTag, parsed, bump, candidate)
	if err != nil {
		return err
	}

	if exists, err := repo.TagExists(next.String()); err != nil {
		return err
	} else if exists {
		return fmt.Errorf("%s already exists, so this release has run before", next)
	}

	docker := dockerPlan{
		images: plan.DockerImages(*dockerImages, *dockerRegistry, os.Getenv("GITHUB_REPOSITORY")),
		tags:   plan.DockerTags(*dockerTags, next),
	}
	// Read from the config rather than asked for, so nobody discovers the
	// requirement by having a release fail on a tool they never named.
	needsSyft := publishers.Has(plan.GoReleaser) && plan.NeedsSyft(*dir)

	return publishPlan(next, latest, haveTag, bump, publishers, docker, needsSyft)
}

// dockerPlan is what the image build needs that depends on the version, and so
// cannot be an action input default.
type dockerPlan struct {
	images string
	tags   string
}

// publishPlan writes the plan out as step outputs and a run summary.
func publishPlan(next, latest plan.Version, haveTag bool, bump plan.Bump,
	p plan.Publishers, docker dockerPlan, needsSyft bool) error {
	previous, rangeSpec := "", "HEAD"
	if haveTag {
		previous = latest.String()
		rangeSpec = previous + "..HEAD"
	}

	outputs := [][2]string{
		{"next", next.String()},
		{"previous", previous},
		{"range", rangeSpec},
		{"prerelease", fmt.Sprintf("%t", next.Prerelease())},
		{"major-alias", next.MajorAlias()},
		{"publish-order", p.String()},
		{"docker-images", docker.images},
		{"docker-tags", docker.tags},
		{"needs-syft", fmt.Sprintf("%t", needsSyft)},
	}

	// One flag per publisher. action.yml declares them in plan.Order and gates
	// each on its own flag, so the sequence lives in one place.
	for _, publisher := range plan.Order {
		outputs = append(outputs,
			[2]string{"publish-" + string(publisher), fmt.Sprintf("%t", p.Has(publisher))})
	}

	for _, o := range outputs {
		if err := actions.Output(o[0], o[1]); err != nil {
			return err
		}
	}

	from := previous
	if from == "" {
		from = "nothing, this is the first"
	}
	kind := "release"
	if next.Prerelease() {
		kind = "release candidate"
	}

	return actions.Summary(fmt.Sprintf(`### %s

| | |
| --- | --- |
| Cutting | %s |
| Previous | %s |
| Bump | %s |
| Kind | %s |
| Publishing via | %s |
`, next, next, from, bump, kind, publishOrder(p)))
}

// publishOrder spells the sequence out, because a caller who listed them in
// another order is entitled to see that the order is quill's, not theirs.
func publishOrder(p plan.Publishers) string {
	if len(p) < 2 {
		return p.String()
	}
	return p.String() + ", in that order"
}

// parseScope reads the dropdown a caller declares. The two labels are the ones
// the workflow_dispatch choice shows, and the short forms are for humans
// running quill by hand.
func parseScope(s string) (candidate bool, err error) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "release":
		return false, nil
	case "release candidate", "candidate", "rc", "prerelease":
		return true, nil
	default:
		return false, fmt.Errorf("unknown scope %q, want \"Release\" or \"Release Candidate\"", s)
	}
}

// guardRef refuses to release from anywhere but the release branch. A tag on a
// topic branch points at history that is not on main, which is unpickable
// afterwards.
func guardRef(ref, required string) error {
	if required == "" {
		return nil
	}
	if ref == "" {
		return fmt.Errorf("-require-ref was set but -ref is empty, so nothing can be checked")
	}
	if ref != required {
		return fmt.Errorf("releases come from %s, and this ran on %s", required, ref)
	}
	return nil
}

// checkArtifacts fails before a tag is cut rather than after. A publisher that
// finds no archive fails anyway, but by then the tag exists and that version
// number is burned.
func checkArtifacts(globs []string) error {
	for _, g := range globs {
		matches, err := filepath.Glob(g)
		if err != nil {
			return fmt.Errorf("bad artifact pattern %q: %w", g, err)
		}
		switch len(matches) {
		case 1:
		case 0:
			return fmt.Errorf("no file matched %q, so there is nothing to publish", g)
		default:
			return fmt.Errorf("%q matched %d files (%s), and publishing needs exactly one",
				g, len(matches), strings.Join(matches, ", "))
		}
	}
	return nil
}
