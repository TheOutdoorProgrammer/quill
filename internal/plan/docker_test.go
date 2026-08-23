package plan

import (
	"strings"
	"testing"
)

func TestDockerImages(t *testing.T) {
	cases := []struct {
		name                           string
		explicit, registry, repository string
		want                           string
	}{
		// github.repository keeps the owner's capitalisation, and a registry
		// path that is not lowercase is rejected by the registry.
		{"lowercases the owner", "", "ghcr.io", "TheOutdoorProgrammer/quill",
			"ghcr.io/theoutdoorprogrammer/quill"},
		{"lowercases the whole name", "", "ghcr.io", "Someone/MixedCaseApp",
			"ghcr.io/someone/mixedcaseapp"},
		{"already lowercase", "", "ghcr.io", "someone/app", "ghcr.io/someone/app"},

		{"explicit wins untouched", "docker.io/MyOrg/Thing", "ghcr.io", "a/b",
			"docker.io/MyOrg/Thing"},
		{"explicit is trimmed", "  ghcr.io/a/b  ", "ghcr.io", "x/y", "ghcr.io/a/b"},

		{"no registry", "", "", "Someone/App", "someone/app"},
		{"trailing slash on the registry", "", "ghcr.io/", "a/b", "ghcr.io/a/b"},
		{"spaces around the registry", "", "  ghcr.io ", "a/b", "ghcr.io/a/b"},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := DockerImages(c.explicit, c.registry, c.repository)
			if got != c.want {
				t.Errorf("DockerImages(%q, %q, %q) = %q, want %q",
					c.explicit, c.registry, c.repository, got, c.want)
			}
		})
	}
}

func TestDockerTagsCarryTheVersionRatherThanTheRef(t *testing.T) {
	v, _ := Parse("v1.4.2")
	got := DockerTags("", v)

	// The tag does not exist yet when the image is built, so metadata-action
	// cannot read the version off the ref.
	for _, want := range []string{
		"type=semver,pattern={{version}},value=v1.4.2",
		"type=semver,pattern={{major}}.{{minor}},value=v1.4.2",
		"type=semver,pattern={{major}},value=v1.4.2",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("tag spec is missing %q:\n%s", want, got)
		}
	}
}

// A candidate must never become what `docker pull` gives you with no tag, nor
// what a deployment resolves.
func TestDockerTagsWithholdMovingTagsFromACandidate(t *testing.T) {
	release, _ := Parse("v1.4.2")
	for _, want := range []string{
		"type=raw,value=latest,enable=true",
		"type=raw,value=production,enable=true",
	} {
		if got := DockerTags("", release); !strings.Contains(got, want) {
			t.Errorf("a release should take %q:\n%s", want, got)
		}
	}

	candidate, _ := Parse("v1.4.2-rc.1")
	for _, want := range []string{
		"type=raw,value=latest,enable=false",
		"type=raw,value=production,enable=false",
	} {
		if got := DockerTags("", candidate); !strings.Contains(got, want) {
			t.Errorf("a candidate must not take %q:\n%s", want, got)
		}
	}
}

// An explicit spec replaces the default outright, so a caller that does not
// want a floating production tag can opt out.
func TestDockerTagsExplicitSpecDropsProduction(t *testing.T) {
	v, _ := Parse("v1.4.2")
	if got := DockerTags("type=semver,pattern={{version}},value=v1.4.2", v); strings.Contains(got, "production") {
		t.Errorf("an explicit spec still carried production:\n%s", got)
	}
}

func TestDockerTagsExplicitWins(t *testing.T) {
	v, _ := Parse("v1.4.2")
	spec := "type=raw,value=nightly"

	if got := DockerTags(spec, v); got != spec {
		t.Errorf("DockerTags = %q, want the caller's %q", got, spec)
	}
	if got := DockerTags("  "+spec+"\n", v); got != spec {
		t.Errorf("DockerTags did not trim: %q", got)
	}
}
