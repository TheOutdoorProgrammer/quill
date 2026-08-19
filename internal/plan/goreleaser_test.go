package plan

import (
	"os"
	"path/filepath"
	"testing"
)

func TestNeedsSyftReadsTheConfig(t *testing.T) {
	tests := []struct {
		name     string
		file     string
		contents string
		want     bool
	}{
		{
			name:     "asks for a bill of materials",
			file:     ".goreleaser.yaml",
			contents: "version: 2\nsboms:\n  - artifacts: archive\n",
			want:     true,
		},
		{
			name:     "the other spelling",
			file:     ".goreleaser.yml",
			contents: "version: 2\nsboms:\n  - artifacts: archive\n",
			want:     true,
		},
		{
			name:     "unprefixed name",
			file:     "goreleaser.yaml",
			contents: "version: 2\nsboms:\n  - artifacts: archive\n",
			want:     true,
		},
		{
			name:     "no bill of materials",
			file:     ".goreleaser.yaml",
			contents: "version: 2\nbuilds:\n  - main: ./cmd/app\n",
			want:     false,
		},
		{
			name:     "commented out",
			file:     ".goreleaser.yaml",
			contents: "version: 2\n# sboms:\n#   - artifacts: archive\n",
			want:     false,
		},
		{
			name:     "a nested key that merely ends in sboms",
			file:     ".goreleaser.yaml",
			contents: "version: 2\ndockers:\n  sboms:\n    - artifacts: archive\n",
			want:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			path := filepath.Join(dir, tt.file)
			if err := os.WriteFile(path, []byte(tt.contents), 0o600); err != nil {
				t.Fatal(err)
			}
			if got := NeedsSyft(dir); got != tt.want {
				t.Errorf("NeedsSyft() = %v, want %v", got, tt.want)
			}
		})
	}
}

// A repository that publishes through something other than GoReleaser has no
// config at all, and must not be told it needs a tool.
func TestNeedsSyftWithoutAConfig(t *testing.T) {
	if NeedsSyft(t.TempDir()) {
		t.Error("NeedsSyft() = true with no GoReleaser config")
	}
}

// The first name GoReleaser would read is the one that decides, so a stale
// second file cannot turn the tool on or off behind the first one's back.
func TestNeedsSyftPrefersTheFirstConfig(t *testing.T) {
	dir := t.TempDir()
	write := func(name, contents string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(dir, name), []byte(contents), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	write(".goreleaser.yaml", "version: 2\n")
	write(".goreleaser.yml", "version: 2\nsboms:\n  - artifacts: archive\n")

	if NeedsSyft(dir) {
		t.Error("NeedsSyft() read a config GoReleaser would have ignored")
	}
}
