package plan

import (
	"os"
	"path/filepath"
	"regexp"
)

// goreleaserConfigs are the names GoReleaser looks for, in its own order.
var goreleaserConfigs = []string{
	".goreleaser.yaml",
	".goreleaser.yml",
	"goreleaser.yaml",
	"goreleaser.yml",
}

// sbomsBlock matches an `sboms:` key at the top level, where GoReleaser reads
// it. Indented, commented and quoted occurrences are not the real thing.
var sbomsBlock = regexp.MustCompile(`(?m)^sboms:`)

// NeedsSyft reports whether a GoReleaser config asks for a bill of materials.
// GoReleaser shells out to syft for one and does not install it, so a caller
// that has to remember a flag finds out by having the release fail.
func NeedsSyft(dir string) bool {
	for _, name := range goreleaserConfigs {
		contents, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			continue
		}
		return sbomsBlock.Match(contents)
	}
	return false
}
