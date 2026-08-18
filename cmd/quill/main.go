// Command quill is the thinking half of the quill release action: it decides
// the version, cuts the tag, moves the major alias, and takes a tag back when
// the release that earned it failed.
package main

import (
	"fmt"
	"os"

	"github.com/TheOutdoorProgrammer/quill/internal/actions"
)

// version is stamped at build time. The committed binaries carry the release
// they were built for, which is how a stale dist announces itself.
var version = "dev"

const usage = `quill decides and cuts a release.

Usage:
  quill plan     work out the next version and publish it as step outputs
  quill tag      create and push the annotated tag
  quill alias    repoint the moving major tag, as in v1
  quill untag    delete a tag after the release that earned it failed
  quill export   put QUILL_EXPORT's KEY=value lines into the job environment
  quill version  print quill's own version

Run a subcommand with -h for its flags.
`

func main() {
	if len(os.Args) < 2 {
		fmt.Fprint(os.Stderr, usage)
		os.Exit(2)
	}

	var err error
	switch os.Args[1] {
	case "plan":
		err = runPlan(os.Args[2:])
	case "tag":
		err = runTag(os.Args[2:])
	case "alias":
		err = runAlias(os.Args[2:])
	case "untag":
		err = runUntag(os.Args[2:])
	case "export":
		err = runExport(os.Args[2:])
	case "version":
		fmt.Println(version)
	case "-h", "--help", "help":
		fmt.Print(usage)
	default:
		fmt.Fprintf(os.Stderr, "quill: unknown subcommand %q\n\n%s", os.Args[1], usage)
		os.Exit(2)
	}

	if err != nil {
		actions.Errorf("%v", err)
		os.Exit(1)
	}
}
