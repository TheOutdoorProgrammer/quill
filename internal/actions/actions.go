// Package actions is the thin layer between quill and the runner: workflow
// commands, step outputs, exported environment, and the run summary.
//
// Every writer falls back to stdout when its file variable is unset, so the
// whole tool runs and can be inspected outside a workflow.
package actions

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"strings"
)

// Out is where workflow commands and fallback writes go. Tests redirect it.
var Out io.Writer = os.Stdout

// emit writes to the runner's log. A failed write there has nowhere left to be
// reported, so the error is deliberately dropped.
func emit(format string, args ...any) {
	_, _ = fmt.Fprintf(Out, format, args...)
}

// escape encodes the characters that would otherwise end a workflow command
// early or split it across lines. Matching @actions/core, which is the only
// specification of this that exists.
var escape = strings.NewReplacer("%", "%25", "\r", "%0D", "\n", "%0A")

func command(name, message string) {
	emit("::%s::%s\n", name, escape.Replace(message))
}

// Errorf writes an annotation the run surfaces as a red error. It does not
// exit, because the caller usually has cleanup to do first.
func Errorf(format string, args ...any) { command("error", fmt.Sprintf(format, args...)) }

// Warnf writes an annotation that does not fail the job.
func Warnf(format string, args ...any) { command("warning", fmt.Sprintf(format, args...)) }

// Noticef writes an informational annotation.
func Noticef(format string, args ...any) { command("notice", fmt.Sprintf(format, args...)) }

// Mask stops a value appearing in the log if something later echoes it.
func Mask(value string) {
	if value != "" {
		command("add-mask", value)
	}
}

// Output publishes a step output. Values are written with a random delimiter
// rather than key=value, because a value containing a newline would otherwise
// let a caller inject arbitrary further outputs.
func Output(key, value string) error { return writeKeyValue("GITHUB_OUTPUT", key, value) }

// Export sets an environment variable for every later step, in this action and
// in the job that called it. This is how a publisher's credentials arrive.
func Export(key, value string) error { return writeKeyValue("GITHUB_ENV", key, value) }

func writeKeyValue(envVar, key, value string) error {
	path := os.Getenv(envVar)
	if path == "" {
		emit("%s: %s=%s\n", envVar, key, value)
		return nil
	}

	delimiter, err := randomDelimiter()
	if err != nil {
		return err
	}
	// A value containing the delimiter could close the block early. It cannot
	// guess 16 random bytes, but refusing costs nothing.
	if strings.Contains(value, delimiter) {
		return fmt.Errorf("value for %s contains its own delimiter", key)
	}

	block := fmt.Sprintf("%s<<%s\n%s\n%s\n", key, delimiter, value, delimiter)
	return appendTo(path, envVar, block)
}

func randomDelimiter() (string, error) {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", fmt.Errorf("generating a delimiter: %w", err)
	}
	return "ghadelimiter_" + hex.EncodeToString(b[:]), nil
}

// Summary appends markdown to the run summary, which is the only part of a
// release a person reads without opening the logs.
func Summary(markdown string) error {
	path := os.Getenv("GITHUB_STEP_SUMMARY")
	if path == "" {
		emit("%s\n", markdown)
		return nil
	}
	return appendTo(path, "GITHUB_STEP_SUMMARY", markdown+"\n")
}

func appendTo(path, name, content string) error {
	f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0o644)
	if err != nil {
		return fmt.Errorf("opening %s: %w", name, err)
	}
	defer func() { _ = f.Close() }()

	if _, err := f.WriteString(content); err != nil {
		return fmt.Errorf("writing %s: %w", name, err)
	}
	return f.Close()
}

// Group opens a collapsible log section and returns the function that closes
// it, so a caller can defer the close next to the open.
func Group(title string) func() {
	command("group", title)
	return func() { emit("::endgroup::\n") }
}
