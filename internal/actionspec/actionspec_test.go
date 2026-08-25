// Package actionspec holds no code. It exists to hold the tests that keep
// action.yml honest about what the Go side believes.
package actionspec

import (
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/TheOutdoorProgrammer/quill/internal/plan"
)

const (
	actionPath         = "../../action.yml"
	appleSigningPath   = "../../apple-signing/action.yml"
	stagedWorkflowPath = "../../.github/workflows/staged-release.yml"
)

func action(t *testing.T) string {
	t.Helper()

	raw, err := os.ReadFile(actionPath)
	if err != nil {
		t.Fatalf("reading the action: %v", err)
	}
	return string(raw)
}

func stagedWorkflow(t *testing.T) string {
	t.Helper()

	raw, err := os.ReadFile(stagedWorkflowPath)
	if err != nil {
		t.Fatalf("reading the staged workflow: %v", err)
	}
	return string(raw)
}

func appleSigningAction(t *testing.T) string {
	t.Helper()

	raw, err := os.ReadFile(appleSigningPath)
	if err != nil {
		t.Fatalf("reading the Apple signing action: %v", err)
	}
	return string(raw)
}

// publishStep is the line that declares one publisher's publishing step.
func publishStep(publisher plan.Publisher) string {
	return fmt.Sprintf("      id: %s\n", publisher)
}

// The sequence lives in plan.Order, and action.yml is where it actually
// happens. Nothing stops the two disagreeing except this.
func TestPublishStepsFollowTheDeclaredOrder(t *testing.T) {
	yaml := action(t)

	positions := make([]int, 0, len(plan.Order))
	for _, publisher := range plan.Order {
		at := strings.Index(yaml, publishStep(publisher))
		if at < 0 {
			t.Fatalf("%s has no publishing step in action.yml", publisher)
		}
		positions = append(positions, at)
	}

	for i := 1; i < len(positions); i++ {
		if positions[i] <= positions[i-1] {
			t.Errorf("%s is declared before %s, but plan.Order runs it after",
				plan.Order[i], plan.Order[i-1])
		}
	}
}

// A publisher that is not gated on its own flag runs for every release, and one
// that is not gated on dry-run publishes from a pull request.
func TestPublishStepsAreGated(t *testing.T) {
	yaml := action(t)

	for _, publisher := range plan.Order {
		start := strings.Index(yaml, publishStep(publisher))
		if start < 0 {
			t.Fatalf("%s has no publishing step in action.yml", publisher)
		}
		// The `if:` sits on the line after the id, so a short window is enough
		// and cannot reach into the next step.
		window := yaml[start:min(start+400, len(yaml))]

		want := fmt.Sprintf("steps.plan.outputs.publish-%s == 'true'", publisher)
		if !strings.Contains(window, want) {
			t.Errorf("%s is not gated on %q:\n%s", publisher, want, window)
		}
		if !strings.Contains(window, "inputs.dry-run != 'true'") {
			t.Errorf("%s would publish during a dry run:\n%s", publisher, window)
		}
	}
}

// Every publisher the Go side knows about needs an input block, or a caller can
// select something the action cannot configure.
func TestEveryPublisherHasInputs(t *testing.T) {
	yaml := action(t)

	for _, publisher := range plan.Order {
		if !strings.Contains(yaml, fmt.Sprintf("\n  %s-", publisher)) {
			t.Errorf("%s has no %s-* inputs declared", publisher, publisher)
		}
	}
}

func TestStagedWorkflowRegistryAuthenticationFallbacks(t *testing.T) {
	yaml := stagedWorkflow(t)

	for _, want := range []string{
		"gcp-workload-identity-provider:",
		"gcp-service-account:",
		"uses: google-github-actions/auth@v3",
		"create_credentials_file: false",
		"secrets.docker-password != '' && secrets.docker-password",
		"steps.gcp-auth.outputs.access_token != '' && steps.gcp-auth.outputs.access_token",
		"|| github.token",
		"inputs.docker-username != '' && inputs.docker-username",
		"steps.gcp-auth.outputs.access_token != '' && 'oauth2accesstoken'",
		"|| github.actor",
	} {
		if !strings.Contains(yaml, want) {
			t.Errorf("staged workflow is missing %q", want)
		}
	}
}

func TestStagedWorkflowRequiresCompleteGCPIdentity(t *testing.T) {
	yaml := stagedWorkflow(t)

	for _, want := range []string{
		"inputs.gcp-workload-identity-provider == '' && inputs.gcp-service-account != ''",
		"inputs.gcp-workload-identity-provider != '' && inputs.gcp-service-account == ''",
		"inputs.gcp-workload-identity-provider != '' && inputs.gcp-service-account != ''",
	} {
		if !strings.Contains(yaml, want) {
			t.Errorf("staged workflow is missing GCP identity guard %q", want)
		}
	}
}

func TestAppleSigningActionKeepsSecretsOutOfRunInterpolation(t *testing.T) {
	yaml := appleSigningAction(t)

	for _, secret := range []string{
		"inputs.p12-file-base64",
		"inputs.p12-password",
		"inputs.api-private-key",
	} {
		if !strings.Contains(yaml, secret) {
			t.Errorf("Apple signing action does not consume %s", secret)
		}
	}

	runBlocks := strings.Split(yaml, "      run: |")
	for _, block := range runBlocks[1:] {
		block = strings.SplitN(block, "\n    - ", 2)[0]
		if strings.Contains(block, "${{ inputs.") {
			t.Errorf("secret-capable input is interpolated into a run block:\n%s", block)
		}
	}
	for _, line := range strings.Split(yaml, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "run:") && strings.Contains(line, "${{ inputs.") {
			t.Errorf("secret-capable input is interpolated into an inline run: %s", line)
		}
	}
}

func TestAppleSigningActionPinsCertificateImporter(t *testing.T) {
	yaml := appleSigningAction(t)
	want := "uses: Apple-Actions/import-codesign-certs@5142e029c445c10ffc7149d172e540235a065466"
	if !strings.Contains(yaml, want) {
		t.Errorf("Apple certificate importer is not pinned to %q", want)
	}
}
