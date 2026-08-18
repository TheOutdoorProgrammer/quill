package gitrepo

import (
	"os/exec"
	"path/filepath"
	"slices"
	"testing"
)

// newTestRepo builds a working tree with a bare remote beside it. Signing and
// the global hooks path are pinned off locally, because a fresh `git init`
// inherits both and would turn these tests into a hardware key prompt.
func newTestRepo(t *testing.T) *Repo {
	t.Helper()

	root := t.TempDir()
	remote := filepath.Join(root, "remote.git")
	work := filepath.Join(root, "work")

	git := func(dir string, args ...string) {
		t.Helper()
		cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}

	if out, err := exec.Command("git", "init", "--bare", "-b", "main", remote).CombinedOutput(); err != nil {
		t.Fatalf("git init --bare: %v\n%s", err, out)
	}
	if out, err := exec.Command("git", "init", "-b", "main", work).CombinedOutput(); err != nil {
		t.Fatalf("git init: %v\n%s", err, out)
	}

	git(work, "config", "commit.gpgsign", "false")
	git(work, "config", "tag.gpgsign", "false")
	git(work, "config", "core.hooksPath", filepath.Join(root, "no-hooks"))
	git(work, "config", "user.name", "quill test")
	git(work, "config", "user.email", "quill@example.invalid")
	git(work, "remote", "add", "origin", remote)

	git(work, "commit", "--allow-empty", "-m", "first")
	git(work, "push", "-u", "origin", "main")

	return New(work, "origin")
}

func TestTagsIsEmptyOnAFreshRepo(t *testing.T) {
	tags, err := newTestRepo(t).Tags()
	if err != nil {
		t.Fatalf("Tags: %v", err)
	}
	if len(tags) != 0 {
		t.Errorf("Tags = %v, want none", tags)
	}
}

func TestTagPushesAndLists(t *testing.T) {
	r := newTestRepo(t)

	if err := r.Tag("v1.0.0", "v1.0.0"); err != nil {
		t.Fatalf("Tag: %v", err)
	}

	tags, err := r.Tags()
	if err != nil {
		t.Fatalf("Tags: %v", err)
	}
	if !slices.Contains(tags, "v1.0.0") {
		t.Errorf("Tags = %v, want it to contain v1.0.0", tags)
	}

	exists, err := r.TagExists("v1.0.0")
	if err != nil || !exists {
		t.Errorf("TagExists = %v, %v; want true, nil", exists, err)
	}
}

func TestDeleteTagRemovesItEverywhere(t *testing.T) {
	r := newTestRepo(t)

	if err := r.Tag("v1.0.0", "v1.0.0"); err != nil {
		t.Fatalf("Tag: %v", err)
	}
	if problems := r.DeleteTag("v1.0.0"); len(problems) != 0 {
		t.Fatalf("DeleteTag: %v", problems)
	}

	exists, err := r.TagExists("v1.0.0")
	if err != nil {
		t.Fatalf("TagExists: %v", err)
	}
	if exists {
		t.Error("the tag survived the delete")
	}
}

// Cleanup runs after a failure, so a problem here must not mask the real one.
func TestDeleteTagReportsRatherThanFails(t *testing.T) {
	problems := newTestRepo(t).DeleteTag("v9.9.9")
	if len(problems) == 0 {
		t.Error("expected the missing tag to be reported")
	}
}

func TestMoveTagRepointsAnExistingAlias(t *testing.T) {
	r := newTestRepo(t)

	if err := r.Tag("v1.0.0", "v1.0.0"); err != nil {
		t.Fatalf("Tag: %v", err)
	}
	if err := r.MoveTag("v1", "HEAD"); err != nil {
		t.Fatalf("MoveTag: %v", err)
	}

	first, err := r.HeadSHA()
	if err != nil {
		t.Fatalf("HeadSHA: %v", err)
	}

	cmd := exec.Command("git", "-C", r.Dir, "commit", "--allow-empty", "-m", "second")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("commit: %v\n%s", err, out)
	}

	// A plain tag call would fail here, which is why MoveTag forces.
	if err := r.MoveTag("v1", "HEAD"); err != nil {
		t.Fatalf("MoveTag over an existing alias: %v", err)
	}

	second, err := r.HeadSHA()
	if err != nil {
		t.Fatalf("HeadSHA: %v", err)
	}
	if first == second {
		t.Fatal("the second commit did not move HEAD, so this proves nothing")
	}

	out, err := exec.Command("git", "-C", r.Dir, "rev-list", "-n", "1", "v1").Output()
	if err != nil {
		t.Fatalf("rev-list: %v", err)
	}
	if got := string(out[:len(second)]); got != second {
		t.Errorf("v1 points at %s, want %s", got, second)
	}
}

func TestShallowIsFalseForAFullClone(t *testing.T) {
	shallow, err := newTestRepo(t).Shallow()
	if err != nil {
		t.Fatalf("Shallow: %v", err)
	}
	if shallow {
		t.Error("a freshly initialised repo reported as shallow")
	}
}

// The default actions/checkout is depth 1. Detecting that is what stops a
// release restarting at v0.1.0 and clobbering the real version line.
func TestShallowIsTrueForADepthLimitedClone(t *testing.T) {
	r := newTestRepo(t)

	if err := r.Tag("v1.0.0", "v1.0.0"); err != nil {
		t.Fatalf("Tag: %v", err)
	}

	origin, err := r.run("remote", "get-url", "origin")
	if err != nil {
		t.Fatalf("remote get-url: %v", err)
	}

	clone := filepath.Join(t.TempDir(), "shallow")
	out, err := exec.Command("git", "clone", "--depth", "1", "file://"+origin, clone).CombinedOutput()
	if err != nil {
		t.Fatalf("shallow clone: %v\n%s", err, out)
	}

	shallow, err := New(clone, "origin").Shallow()
	if err != nil {
		t.Fatalf("Shallow: %v", err)
	}
	if !shallow {
		t.Error("a depth-limited clone was not detected as shallow")
	}
}
