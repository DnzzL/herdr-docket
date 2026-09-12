package fleet

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// readAgent returns an agent's raw AGENT.md, failing the test rather than
// carrying an empty string forward into a content assertion.
func readAgent(t *testing.T, dir, name string) string {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(dir, "agents", name, "AGENT.md"))
	if err != nil {
		t.Fatal(err)
	}
	return string(raw)
}

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

func TestLoadAgentsParsesDisabled(t *testing.T) {
	dir := t.TempDir()
	writeAgent(t, dir, "parked", "---\nworkdir: "+dir+"\ndisabled: true\n---\nB")
	writeAgent(t, dir, "active", "---\nworkdir: "+dir+"\n---\nB")
	agents, diags := LoadAgents(dir)
	if len(diags) != 0 {
		t.Fatalf("unexpected diagnostics: %v", diags)
	}
	if !agents["parked"].Disabled {
		t.Fatal("disabled: true must load as Disabled")
	}
	if agents["active"].Disabled {
		t.Fatal("an absent disabled key must load as false")
	}
}

func TestSetDisabledPreservesCommentsAndPersona(t *testing.T) {
	dir := t.TempDir()
	original := `---
model: claude-sonnet-5
workdir: ` + dir + `
workspace: root # root works on the checkout
---

You are the dev persona.
Multi-line, untouched.
`
	writeAgent(t, dir, "dev", original)

	if err := SetDisabled(dir, "dev", true); err != nil {
		t.Fatal(err)
	}
	agents, diags := LoadAgents(dir)
	if len(diags) != 0 {
		t.Fatalf("diagnostics: %v", diags)
	}
	if !agents["dev"].Disabled {
		t.Fatal("expected Disabled after pause")
	}
	if agents["dev"].Model != "claude-sonnet-5" || agents["dev"].Workspace != "root" {
		t.Fatalf("other keys lost: %+v", agents["dev"])
	}
	body := readAgent(t, dir, "dev")
	if !strings.Contains(body, "# root works on the checkout") {
		t.Fatalf("frontmatter comment destroyed:\n%s", body)
	}
	if !strings.Contains(body, "You are the dev persona.\nMulti-line, untouched.") {
		t.Fatalf("persona destroyed:\n%s", body)
	}

	// Toggling back must leave the file exactly as it was found.
	if err := SetDisabled(dir, "dev", false); err != nil {
		t.Fatal(err)
	}
	if got := readAgent(t, dir, "dev"); got != original {
		t.Fatalf("resume did not restore the file:\n%s", got)
	}
	agents, _ = LoadAgents(dir)
	if agents["dev"].Disabled {
		t.Fatal("expected enabled after resume")
	}
}

func TestSetDisabledIdempotentAndUnknownAgent(t *testing.T) {
	dir := t.TempDir()
	writeAgent(t, dir, "dev", "---\nworkdir: "+dir+"\n---\nB")
	if err := SetDisabled(dir, "dev", true); err != nil {
		t.Fatal(err)
	}
	if err := SetDisabled(dir, "dev", true); err != nil {
		t.Fatal(err)
	}
	if n := strings.Count(readAgent(t, dir, "dev"), "disabled:"); n != 1 {
		t.Fatalf("want one disabled line, got %d", n)
	}
	if err := SetDisabled(dir, "ghost", true); err == nil {
		t.Fatal("unknown agent must error")
	}
}

// A hand-written `disabled: false` is a line SetDisabled owns, not a second
// key to append beside it.
func TestSetDisabledRewritesAHandWrittenDisabledLine(t *testing.T) {
	dir := t.TempDir()
	writeAgent(t, dir, "dev", "---\nworkdir: "+dir+"\ndisabled: false\n---\nB")
	agents, _ := LoadAgents(dir)
	if agents["dev"].Disabled {
		t.Fatal("disabled: false must load as enabled")
	}
	if err := SetDisabled(dir, "dev", true); err != nil {
		t.Fatal(err)
	}
	if n := strings.Count(readAgent(t, dir, "dev"), "disabled:"); n != 1 {
		t.Fatalf("want the one line rewritten, not appended: got %d", n)
	}
	agents, _ = LoadAgents(dir)
	if !agents["dev"].Disabled {
		t.Fatal("expected Disabled after pause")
	}
}

func TestLoadAgentsMissingDirIsEmptyNotError(t *testing.T) {
	agents, diags := LoadAgents(filepath.Join(t.TempDir(), "nope"))
	if len(agents) != 0 || len(diags) != 0 {
		t.Fatalf("want empty, got %v %v", agents, diags)
	}
}

// The reader (splitFrontmatter) and the line editor (frontmatterRange) once
// decided the header's end by two different rules, so a file one accepted the
// other could reject — a pause that silently does nothing, or a persona read
// as YAML. One rule now decides the span; this holds both helpers to it across
// the edges where they used to differ.
func TestFrontmatterHelpersAgree(t *testing.T) {
	for _, tc := range []struct {
		name string
		raw  string
	}{
		{"plain", "---\nworkdir: /x\n---\nBody."},
		{"trailing newline", "---\nworkdir: /x\n---\nBody.\n"},
		{"empty body", "---\nworkdir: /x\n---\n"},
		{"no body at all", "---\nworkdir: /x\n---"},
		{"multi-line header", "---\nmodel: x\nworkdir: /x\n---\nBody."},
		{"space after opening fence", "--- \nworkdir: /x\n---\nBody."},
		{"space after closing fence", "---\nworkdir: /x\n--- \nBody."},
		{"crlf", "---\r\nworkdir: /x\r\n---\r\nBody.\r\n"},
		{"no frontmatter", "hello\n---\nBody."},
		{"unterminated", "---\nworkdir: /x\nBody."},
		{"only opening fence", "---"},
		{"closing fence with junk", "---\nworkdir: /x\n---junk\nBody."},
		{"empty", ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			front, _, err := splitFrontmatter(tc.raw)
			lines := strings.Split(tc.raw, "\n")
			start, end, ok := frontmatterRange(lines)
			switch {
			case ok && err != nil:
				t.Fatalf("frontmatterRange found lines %d:%d but splitFrontmatter failed: %v", start, end, err)
			case !ok && err == nil:
				t.Fatalf("frontmatterRange found no span but splitFrontmatter returned %q", front)
			case ok:
				if want := strings.Join(lines[start:end], "\n"); front != want {
					t.Fatalf("front = %q, want the range's %q", front, want)
				}
			}
		})
	}
}

// The wording is the loader's contract with a human fixing their AGENT.md, so
// the two failures stay distinct even though one function now reports both.
func TestSplitFrontmatterErrorWordingKeepsItsMeaning(t *testing.T) {
	for _, tc := range []struct {
		name, raw, want string
	}{
		{"missing", "no fences here", "no frontmatter"},
		{"unterminated", "---\nworkdir: /x\n", "unterminated"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, _, err := splitFrontmatter(tc.raw)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("err = %v, want it to mention %q", err, tc.want)
			}
		})
	}
}
