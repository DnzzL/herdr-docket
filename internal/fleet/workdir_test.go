package fleet

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Resuming is the gesture that puts an agent back in front of real work, and
// `init --factory` ships three agents pointed at a placeholder repo with
// "point its workdir at your repo, then resume" printed beside them. Nothing
// enforced the "then": an agent could be armed at a directory that does not
// exist, and the first task routed to it failed at provision with nothing on
// the board explaining why.
func TestResumableRefusesAnAgentPointedNowhere(t *testing.T) {
	dir := t.TempDir()
	real := filepath.Join(dir, "repo")
	if err := os.MkdirAll(real, 0o755); err != nil {
		t.Fatal(err)
	}
	writeAgent(t, dir, "ok", "---\nworkdir: "+real+"\n---\nP\n")
	writeAgent(t, dir, "nowhere", "---\nworkdir: "+filepath.Join(dir, "gone")+"\n---\nP\n")

	if err := Resumable(dir, "ok"); err != nil {
		t.Errorf("an agent pointed at a real checkout resumes: %v", err)
	}

	err := Resumable(dir, "nowhere")
	if err == nil {
		t.Fatal("an agent pointed at a missing directory must not resume")
	}
	// The message has to name the path, because the fix is to edit that line.
	if got := err.Error(); !strings.Contains(got, filepath.Join(dir, "gone")) || !strings.Contains(got, "workdir") {
		t.Errorf("the refusal must name the workdir to edit: %v", err)
	}

	// An agent the fleet does not have is the loader's error, not this one's.
	if err := Resumable(dir, "absent"); err == nil {
		t.Error("resuming an unknown agent must fail")
	}
}
