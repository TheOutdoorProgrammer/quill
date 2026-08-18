package main

import "testing"

func TestParseExport(t *testing.T) {
	raw := `
# a comment
HOMEBREW_TAP_GITHUB_TOKEN=ghs_example
EMPTY=
SPACED = value with spaces
BASE64=aGVsbG8=d29ybGQ=
`
	got, err := parseExport(raw)
	if err != nil {
		t.Fatalf("parseExport: %v", err)
	}

	want := [][2]string{
		{"HOMEBREW_TAP_GITHUB_TOKEN", "ghs_example"},
		{"EMPTY", ""},
		{"SPACED", " value with spaces"},
		// Splitting on the first = only, or a padded base64 value loses its tail.
		{"BASE64", "aGVsbG8=d29ybGQ="},
	}
	if len(got) != len(want) {
		t.Fatalf("parseExport returned %d pairs, want %d: %v", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("pair %d = %v, want %v", i, got[i], want[i])
		}
	}
}

func TestParseExportIgnoresNothing(t *testing.T) {
	got, err := parseExport("")
	if err != nil {
		t.Fatalf("parseExport: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("parseExport(\"\") = %v, want none", got)
	}
}

func TestParseExportRejectsBadLines(t *testing.T) {
	bad := map[string]string{
		"no separator":     "JUST_A_NAME",
		"empty name":       "=value",
		"name with a dash": "MY-VAR=value",
		"name with a dot":  "my.var=value",
		"leading digit":    "1VAR=value",
	}
	for name, raw := range bad {
		if _, err := parseExport(raw); err == nil {
			t.Errorf("%s: parseExport(%q) was accepted, want an error", name, raw)
		}
	}
}
