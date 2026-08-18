package actions

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func capture(t *testing.T) *bytes.Buffer {
	t.Helper()
	var buf bytes.Buffer
	previous := Out
	Out = &buf
	t.Cleanup(func() { Out = previous })
	return &buf
}

// A newline in an annotation would end the command early and drop the rest.
func TestWorkflowCommandsEscapeTheirMessage(t *testing.T) {
	buf := capture(t)
	Errorf("first\nsecond 100%% done")

	got := buf.String()
	if strings.Count(got, "\n") != 1 {
		t.Errorf("the message broke across lines: %q", got)
	}
	for _, want := range []string{"%0A", "%25"} {
		if !strings.Contains(got, want) {
			t.Errorf("output %q is missing the %s escape", got, want)
		}
	}
}

func TestMaskIgnoresAnEmptyValue(t *testing.T) {
	buf := capture(t)
	Mask("")
	if buf.Len() != 0 {
		t.Errorf("masking nothing wrote %q", buf.String())
	}

	Mask("hunter2")
	if !strings.Contains(buf.String(), "::add-mask::hunter2") {
		t.Errorf("mask not emitted: %q", buf.String())
	}
}

// A multi-line value written as key=value would let the rest of it be read as
// further outputs, so the delimiter form is the security property here.
func TestOutputSurvivesAMultiLineValue(t *testing.T) {
	path := filepath.Join(t.TempDir(), "outputs")
	t.Setenv("GITHUB_OUTPUT", path)

	value := "line one\nnext=injected"
	if err := Output("notes", value); err != nil {
		t.Fatalf("Output: %v", err)
	}

	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading outputs: %v", err)
	}
	got := string(raw)

	if !strings.HasPrefix(got, "notes<<ghadelimiter_") {
		t.Errorf("output did not use a delimiter block: %q", got)
	}
	if !strings.Contains(got, value) {
		t.Errorf("output %q lost the value", got)
	}
}

func TestOutputAppendsRatherThanReplaces(t *testing.T) {
	path := filepath.Join(t.TempDir(), "outputs")
	t.Setenv("GITHUB_OUTPUT", path)

	if err := Output("next", "v1.2.3"); err != nil {
		t.Fatalf("Output: %v", err)
	}
	if err := Output("previous", "v1.2.2"); err != nil {
		t.Fatalf("Output: %v", err)
	}

	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading outputs: %v", err)
	}
	for _, want := range []string{"next<<", "v1.2.3", "previous<<", "v1.2.2"} {
		if !strings.Contains(string(raw), want) {
			t.Errorf("outputs are missing %q: %s", want, raw)
		}
	}
}

// Outside a workflow every writer falls back to stdout, so the tool can be run
// and inspected by hand.
func TestWritersFallBackToStdoutWithNoEnvironment(t *testing.T) {
	t.Setenv("GITHUB_OUTPUT", "")
	t.Setenv("GITHUB_STEP_SUMMARY", "")
	buf := capture(t)

	if err := Output("next", "v1.2.3"); err != nil {
		t.Fatalf("Output: %v", err)
	}
	if err := Summary("### heading"); err != nil {
		t.Fatalf("Summary: %v", err)
	}

	got := buf.String()
	if !strings.Contains(got, "next=v1.2.3") || !strings.Contains(got, "### heading") {
		t.Errorf("fallback output was %q", got)
	}
}

func TestGroupClosesItself(t *testing.T) {
	buf := capture(t)
	end := Group("Publishing")
	end()

	got := buf.String()
	if !strings.Contains(got, "::group::Publishing") || !strings.Contains(got, "::endgroup::") {
		t.Errorf("group markers missing: %q", got)
	}
}
