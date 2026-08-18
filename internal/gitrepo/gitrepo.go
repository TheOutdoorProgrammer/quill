// Package gitrepo is the git plumbing a release needs: reading the tag list,
// cutting a tag, and taking one back when the release that earned it failed.
package gitrepo

import (
	"bytes"
	"fmt"
	"os/exec"
	"strings"
)

// Repo is a working tree. Every call names the directory explicitly rather
// than relying on the process working directory, which nothing else here owns.
type Repo struct {
	Dir    string
	Remote string
}

// New opens dir. An empty remote means "origin", which is what a checkout on a
// runner always has.
func New(dir, remote string) *Repo {
	if remote == "" {
		remote = "origin"
	}
	return &Repo{Dir: dir, Remote: remote}
}

func (r *Repo) run(args ...string) (string, error) {
	cmd := exec.Command("git", append([]string{"-C", r.Dir}, args...)...)

	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr

	if err := cmd.Run(); err != nil {
		detail := strings.TrimSpace(stderr.String())
		if detail == "" {
			detail = err.Error()
		}
		return "", fmt.Errorf("git %s: %s", strings.Join(args, " "), detail)
	}
	return strings.TrimSpace(stdout.String()), nil
}

// Tags lists every tag that looks like a version. Filtering happens in the
// version parser rather than here, so an unexpected shape is dropped in one
// place with one rule.
func (r *Repo) Tags() ([]string, error) {
	out, err := r.run("tag", "--list", "v*")
	if err != nil {
		return nil, err
	}
	if out == "" {
		return nil, nil
	}
	return strings.Split(out, "\n"), nil
}

// Shallow reports whether the checkout was cloned with a depth limit. A
// shallow clone has no tags at all, so a release computed from one would
// silently restart at v0.1.0 and clobber the real version line.
func (r *Repo) Shallow() (bool, error) {
	out, err := r.run("rev-parse", "--is-shallow-repository")
	if err != nil {
		return false, err
	}
	return out == "true", nil
}

// HeadSHA is the commit a tag would point at.
func (r *Repo) HeadSHA() (string, error) { return r.run("rev-parse", "HEAD") }

// SetIdentity gives the tagger a name. Runners have no git identity, and an
// annotated tag will not be created without one.
func (r *Repo) SetIdentity(name, email string) error {
	if _, err := r.run("config", "user.name", name); err != nil {
		return err
	}
	_, err := r.run("config", "user.email", email)
	return err
}

// Tag creates an annotated tag and pushes it.
func (r *Repo) Tag(name, message string) error {
	if _, err := r.run("tag", "-a", name, "-m", message); err != nil {
		return err
	}
	_, err := r.run("push", r.Remote, name)
	return err
}

// MoveTag repoints an existing tag and force pushes it. This is how a major
// alias such as v1 tracks the newest release, the way actions/checkout@v4
// does, so callers can pin a major and still get fixes.
func (r *Repo) MoveTag(name, target string) error {
	if _, err := r.run("tag", "--force", name, target); err != nil {
		return err
	}
	_, err := r.run("push", "--force", r.Remote, name)
	return err
}

// DeleteTag removes a tag locally and remotely, reporting what it could not
// do rather than failing. It runs as cleanup after something already went
// wrong, and an error here would replace the real one.
func (r *Repo) DeleteTag(name string) []error {
	var problems []error
	if _, err := r.run("push", "--delete", r.Remote, name); err != nil {
		problems = append(problems, err)
	}
	if _, err := r.run("tag", "--delete", name); err != nil {
		problems = append(problems, err)
	}
	return problems
}

// TagExists reports whether a tag is already present locally.
func (r *Repo) TagExists(name string) (bool, error) {
	out, err := r.run("tag", "--list", name)
	if err != nil {
		return false, err
	}
	return out != "", nil
}
