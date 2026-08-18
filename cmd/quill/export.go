package main

import (
	"flag"
	"fmt"
	"os"
	"regexp"
	"strings"

	"github.com/TheOutdoorProgrammer/quill/internal/actions"
)

// Values arrive through the environment rather than argv, because a publisher's
// credentials would otherwise be visible in the process list.
const exportVar = "QUILL_EXPORT"

var envKey = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

func runExport(args []string) error {
	fs := flag.NewFlagSet("export", flag.ExitOnError)
	if err := fs.Parse(args); err != nil {
		return err
	}

	pairs, err := parseExport(os.Getenv(exportVar))
	if err != nil {
		return err
	}

	var names []string
	for _, p := range pairs {
		if err := actions.Export(p[0], p[1]); err != nil {
			return err
		}
		names = append(names, p[0])
	}

	// Names only. Printing a value here would defeat the point of the variable.
	if len(names) > 0 {
		actions.Noticef("exported %s", strings.Join(names, ", "))
	}
	return nil
}

func parseExport(raw string) ([][2]string, error) {
	var pairs [][2]string

	for i, line := range strings.Split(raw, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		key, value, found := strings.Cut(line, "=")
		if !found {
			return nil, fmt.Errorf("env line %d is not KEY=value", i+1)
		}

		key = strings.TrimSpace(key)
		if !envKey.MatchString(key) {
			return nil, fmt.Errorf("env line %d has an unusable name %q", i+1, key)
		}
		pairs = append(pairs, [2]string{key, value})
	}
	return pairs, nil
}
