package factory

import (
	"os"
	"strings"
	"testing"
)

// The manual and the shipped bytes must not drift: docs/examples.md shows the
// personas a fleet copies by hand, and these are the ones init writes, and a
// manual that describes a loop the binary does not install is the failure
// mode this whole task exists to prevent. Read against the docs on disk — if
// someone edits one side, this goes red until both agree.
func TestWhatWeShipIsWhatTheDocsShow(t *testing.T) {
	examples, err := os.ReadFile("../../docs/examples.md")
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range personas {
		if !strings.Contains(string(examples), p.body) {
			t.Errorf("persona %s is not in docs/examples.md verbatim — one of the two has drifted", p.name)
		}
	}

	factory, err := os.ReadFile("../../docs/factory.md")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(factory), "automations:\n"+entries) {
		t.Error("the automations entries in docs/factory.md do not match the ones init writes")
	}
}
