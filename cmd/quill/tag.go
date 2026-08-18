package main

import (
	"flag"
	"fmt"

	"github.com/TheOutdoorProgrammer/quill/internal/actions"
	"github.com/TheOutdoorProgrammer/quill/internal/gitrepo"
	"github.com/TheOutdoorProgrammer/quill/internal/plan"
)

func runTag(args []string) error {
	fs := flag.NewFlagSet("tag", flag.ExitOnError)
	dir := fs.String("dir", ".", "the working tree to tag")
	remote := fs.String("remote", "origin", "the remote to push to")
	tag := fs.String("version", "", "the tag to cut, for example v1.2.0")
	name := fs.String("actor-name", "github-actions[bot]", "tagger name")
	email := fs.String("actor-email",
		"github-actions[bot]@users.noreply.github.com", "tagger email")

	if err := fs.Parse(args); err != nil {
		return err
	}

	v, err := requireVersion(*tag)
	if err != nil {
		return err
	}

	repo := gitrepo.New(*dir, *remote)
	if err := repo.SetIdentity(*name, *email); err != nil {
		return err
	}
	if err := repo.Tag(v.String(), v.String()); err != nil {
		return err
	}

	actions.Noticef("tagged %s", v)
	return nil
}

func runAlias(args []string) error {
	fs := flag.NewFlagSet("alias", flag.ExitOnError)
	dir := fs.String("dir", ".", "the working tree to tag")
	remote := fs.String("remote", "origin", "the remote to push to")
	tag := fs.String("version", "", "the release the alias should point at")
	target := fs.String("target", "HEAD", "what the alias points at")

	if err := fs.Parse(args); err != nil {
		return err
	}

	v, err := requireVersion(*tag)
	if err != nil {
		return err
	}

	// A candidate must never move the alias, or everyone pinned to @v1 is
	// silently upgraded to a version that exists to be rehearsed.
	if v.Prerelease() {
		actions.Noticef("%s is a candidate, so %s stays where it is", v, v.MajorAlias())
		return nil
	}

	repo := gitrepo.New(*dir, *remote)
	if err := repo.MoveTag(v.MajorAlias(), *target); err != nil {
		return err
	}

	actions.Noticef("%s now points at %s", v.MajorAlias(), v)
	return nil
}

// runUntag is cleanup after a failed release, so it reports what it could not
// do and still succeeds. A tag left behind burns that version number forever.
func runUntag(args []string) error {
	fs := flag.NewFlagSet("untag", flag.ExitOnError)
	dir := fs.String("dir", ".", "the working tree to clean up")
	remote := fs.String("remote", "origin", "the remote to clean up")
	tag := fs.String("version", "", "the tag to take back")

	if err := fs.Parse(args); err != nil {
		return err
	}

	v, err := requireVersion(*tag)
	if err != nil {
		return err
	}

	for _, problem := range gitrepo.New(*dir, *remote).DeleteTag(v.String()) {
		actions.Warnf("could not fully remove %s: %v", v, problem)
	}

	actions.Noticef("took back %s so the next attempt can reuse it", v)
	return nil
}

func requireVersion(s string) (plan.Version, error) {
	if s == "" {
		return plan.Version{}, fmt.Errorf("-version is required")
	}
	v, ok := plan.Parse(s)
	if !ok {
		return plan.Version{}, fmt.Errorf("%q is not a version quill understands", s)
	}
	return v, nil
}
