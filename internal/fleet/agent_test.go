package fleet

import (
	"os"
	"path/filepath"
	"testing"
)

func writeAgent(t *testing.T, dir, name, content string) {
	t.Helper()
	p := filepath.Join(dir, "agents", name)
	if err := os.MkdirAll(p, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(p, "AGENT.md"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestLoadAgentsParsesFrontmatterAndBody(t *testing.T) {
	dir := t.TempDir()
	writeAgent(t, dir, "dishnow-marketing", `---
model: claude-sonnet-5
workdir: `+dir+`
workspace: root
skills: ["copywriting"]
---

You run the publication strategy.
`)
	agents, diags := LoadAgents(dir)
	if len(diags) != 0 {
		t.Fatalf("unexpected diagnostics: %v", diags)
	}
	a, ok := agents["dishnow-marketing"]
	if !ok {
		t.Fatalf("agent not loaded, got %v", agents)
	}
	if a.Model != "claude-sonnet-5" || a.Workdir != dir || a.Workspace != "root" {
		t.Fatalf("bad fields: %+v", a)
	}
	if a.Persona != "You run the publication strategy." {
		t.Fatalf("persona = %q", a.Persona)
	}
}

func TestLoadAgentsDefaults(t *testing.T) {
	dir := t.TempDir()
	writeAgent(t, dir, "worker", "---\nworkdir: "+dir+"\n---\nBody.")
	agents, _ := LoadAgents(dir)
	a := agents["worker"]
	if a.Kind != "claude" || a.Workspace != "worktree" || a.TimeoutMinutes != 60 {
		t.Fatalf("defaults not applied: %+v", a)
	}
}

func TestLoadAgentsExpandsTildeWorkdir(t *testing.T) {
	dir := t.TempDir()
	writeAgent(t, dir, "w", "---\nworkdir: ~/somewhere\n---\nB")
	agents, _ := LoadAgents(dir)
	home, _ := os.UserHomeDir()
	if got := agents["w"].Workdir; got != filepath.Join(home, "somewhere") {
		t.Fatalf("workdir = %q", got)
	}
}

func TestLoadAgentsReportsBrokenEntriesAndKeepsTheRest(t *testing.T) {
	dir := t.TempDir()
	writeAgent(t, dir, "good", "---\nworkdir: "+dir+"\n---\nB")
	writeAgent(t, dir, "no-workdir", "---\nmodel: x\n---\nB")
	writeAgent(t, dir, "bad-yaml", "---\nmodel: [unclosed\n---\nB")
	writeAgent(t, dir, "bad-mode", "---\nworkdir: "+dir+"\nworkspace: sandbox\n---\nB")
	agents, diags := LoadAgents(dir)
	if len(agents) != 1 || agents["good"].Name != "good" {
		t.Fatalf("agents = %v", agents)
	}
	if len(diags) != 3 {
		t.Fatalf("want 3 diagnostics, got %v", diags)
	}
}

func TestLoadAgentsMissingDirIsEmptyNotError(t *testing.T) {
	agents, diags := LoadAgents(filepath.Join(t.TempDir(), "nope"))
	if len(agents) != 0 || len(diags) != 0 {
		t.Fatalf("want empty, got %v %v", agents, diags)
	}
}
