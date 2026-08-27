// Package actionspec holds no code. It exists to hold the tests that keep
// action.yml honest about what the Go side believes.
package actionspec

import (
	"crypto/sha256"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/TheOutdoorProgrammer/quill/internal/plan"
)

const (
	actionPath             = "../../action.yml"
	appleSigningPath       = "../../apple-signing/action.yml"
	appleSigningScriptPath = "../../apple-signing/run.sh"
	appleWWDRG3Path        = "../../apple-signing/AppleWWDRCAG3.pem"
	stagedWorkflowPath     = "../../.github/workflows/staged-release.yml"
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

func appleSigningScript(t *testing.T) string {
	t.Helper()

	raw, err := os.ReadFile(appleSigningScriptPath)
	if err != nil {
		t.Fatalf("reading the Apple signing script: %v", err)
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

func TestAppleSigningActionUsesTheRunnerAccountHomeForKeychainAccess(t *testing.T) {
	yaml := appleSigningAction(t)

	for _, want := range []string{
		`runner_home="$(/usr/bin/dscl . -read "/Users/$(/usr/bin/id -un)" NFSHomeDirectory`,
		`echo "home=$runner_home"`,
		"HOME: ${{ steps.configuration.outputs.home }}",
	} {
		if !strings.Contains(yaml, want) {
			t.Errorf("Apple signing action is missing %q", want)
		}
	}

	if got := strings.Count(yaml, "HOME: ${{ steps.configuration.outputs.home }}"); got != 2 {
		t.Errorf("Apple signing action configures HOME for %d Keychain steps, want 2", got)
	}
}

func TestAppleSigningActionExposesTheTemporaryKeychainPassword(t *testing.T) {
	yaml := appleSigningAction(t)
	for _, want := range []string{
		"id: certificates",
		"value: ${{ steps.certificates.outputs.keychain-password }}",
		"SIGNING_KEYCHAIN_PASSWORD: ${{ steps.certificates.outputs.keychain-password }}",
	} {
		if !strings.Contains(yaml, want) {
			t.Errorf("Apple signing action is missing %q", want)
		}
	}

	script := appleSigningScript(t)
	if want := `security unlock-keychain -p "$SIGNING_KEYCHAIN_PASSWORD" "$SIGNING_KEYCHAIN"`; !strings.Contains(script, want) {
		t.Errorf("Apple signing script is missing %q", want)
	}
}

func TestAppleSigningActionPinsTheWWDRG3Intermediate(t *testing.T) {
	raw, err := os.ReadFile(appleWWDRG3Path)
	if err != nil {
		t.Fatalf("reading the Apple WWDR G3 intermediate: %v", err)
	}
	block, rest := pem.Decode(raw)
	if block == nil || block.Type != "CERTIFICATE" || len(rest) != 0 {
		t.Fatal("Apple WWDR G3 intermediate is not one PEM certificate")
	}
	certificate, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		t.Fatalf("parsing the Apple WWDR G3 intermediate: %v", err)
	}
	wantFingerprint := "DCF21878C77F4198E4B4614F03D696D89C66C66008D4244E1B99161AAC91601F"
	if got := fmt.Sprintf("%X", sha256.Sum256(certificate.Raw)); got != wantFingerprint {
		t.Errorf("Apple WWDR G3 fingerprint is %s, want %s", got, wantFingerprint)
	}

	script := appleSigningScript(t)

	for _, want := range []string{
		`security import "$here/AppleWWDRCAG3.pem"`,
		`-k "$SIGNING_KEYCHAIN"`,
		"-f pemseq",
		wantFingerprint,
		"security verify-cert",
		"-p codeSign",
	} {
		if !strings.Contains(script, want) {
			t.Errorf("Apple signing script is missing %q", want)
		}
	}
}
