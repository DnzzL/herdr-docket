package prompt

import (
	"github.com/DnzzL/herdr-docket/internal/fleet"
	"github.com/DnzzL/herdr-docket/internal/work"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Every verb of the protocol is useless if the agent cannot run the binary.
// The plugin is not on a pane's PATH, so an hour of good work ended in
// `herdr-docket: command not found` and a run recorded as silence — the
// fleet's dominant failure, and never the agent's fault. The binary that
// assembles the prompt knows where it lives, so the prompt names an
// invocation that works.
func TestCLINamesAnInvocationThatWorks(t *testing.T) {
	dir := t.TempDir()
	exe := filepath.Join(dir, "herdr-docket")
	if err := os.WriteFile(exe, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}

	t.Run("the absolute path when the bare name is not on PATH", func(t *testing.T) {
		t.Setenv("PATH", t.TempDir())
		if got := cliFor(exe); got != exe {
			t.Errorf("cliFor = %q, want %q", got, exe)
		}
	})

	t.Run("the bare name when PATH already finds it", func(t *testing.T) {
		t.Setenv("PATH", dir)
		if got := cliFor(exe); got != "herdr-docket" {
			t.Errorf("cliFor = %q, want the bare name", got)
		}
	})

	t.Run("the bare name when the binary cannot say where it is", func(t *testing.T) {
		t.Setenv("PATH", t.TempDir())
		if got := cliFor(""); got != "herdr-docket" {
			t.Errorf("cliFor = %q, want the bare name", got)
		}
	})
}

// Commands are rewritten; prose is not. An agent copies the indented lines,
// and a sentence that reads "/home/thomas/.../bin/herdr-docket CLI" helps
// nobody.
func TestOnlyTheLinesAnAgentCopiesAreRewritten(t *testing.T) {
	const src = "Your own task answers to the herdr-docket CLI and nothing else.\n\n  herdr-docket task done X --note \"y\"\n"
	got := runnable(src, "/opt/bin/herdr-docket")

	if !strings.Contains(got, "  /opt/bin/herdr-docket task done") {
		t.Errorf("the command must be runnable:\n%s", got)
	}
	if !strings.Contains(got, "the herdr-docket CLI and nothing else") {
		t.Errorf("prose must keep the plain name:\n%s", got)
	}
	if same := runnable(src, "herdr-docket"); same != src {
		t.Errorf("a CLI already on PATH changes nothing:\n%s", same)
	}
}

// End to end: the prompt an agent actually receives must contain no command
// it cannot run. This is the regression that matters — the protocol is only
// as real as its most-copied line.
func TestTheAssembledPromptContainsNoUnrunnableCommand(t *testing.T) {
	t.Setenv("PATH", t.TempDir()) // herdr-docket is not on a pane's PATH
	fleetDir := t.TempDir()

	out := Runnable(Assemble(
		fleet.Agent{Name: "a", Persona: "P"},
		work.Task{ID: "docket/TASK-1", Title: "t", Open: true, Body: "b"},
		fleetDir, fleet.Words{},
	))
	for _, line := range strings.Split(out, "\n") {
		if strings.HasPrefix(line, "  herdr-docket ") {
			t.Errorf("an agent cannot run this line: %q", line)
		}
	}
	if !strings.Contains(out, "task done") {
		t.Fatalf("the closing verb vanished from the prompt:\n%s", out)
	}
}
