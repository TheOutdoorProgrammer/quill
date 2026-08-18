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

const actionPath = "../../action.yml"

func action(t *testing.T) string {
	t.Helper()

	raw, err := os.ReadFile(actionPath)
	if err != nil {
		t.Fatalf("reading the action: %v", err)
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
