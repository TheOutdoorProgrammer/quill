package plan

import (
	"fmt"
	"strings"
)

// DockerImages resolves what to name the image, an explicit value winning.
// The default lowercases the repository, because a registry path must be
// lowercase and github.repository keeps the owner's capitalisation.
func DockerImages(explicit, registry, repository string) string {
	if explicit = strings.TrimSpace(explicit); explicit != "" {
		return explicit
	}

	name := strings.ToLower(strings.Trim(repository, "/"))
	if registry = strings.Trim(strings.TrimSpace(registry), "/"); registry == "" {
		return name
	}
	return registry + "/" + name
}

// DockerTags resolves the docker/metadata-action tag spec, an explicit value
// winning. The version is passed as `value=` rather than read from the ref,
// because quill has not created the tag yet when the image is built.
func DockerTags(explicit string, v Version) string {
	if explicit = strings.TrimSpace(explicit); explicit != "" {
		return explicit
	}

	// metadata-action drops major and minor tags for a prerelease on its own.
	// `latest` is raw, so it needs saying: a candidate must never become what
	// `docker pull` gives you with no tag.
	return strings.Join([]string{
		fmt.Sprintf("type=semver,pattern={{version}},value=%s", v),
		fmt.Sprintf("type=semver,pattern={{major}}.{{minor}},value=%s", v),
		fmt.Sprintf("type=semver,pattern={{major}},value=%s", v),
		fmt.Sprintf("type=raw,value=latest,enable=%t", !v.Prerelease()),
	}, "\n")
}
