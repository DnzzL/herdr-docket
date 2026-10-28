package backlogmd

import (
	"os"
	"os/exec"
	"testing"
)

// A ticket is a Backlog.md subtask: the real CLI is the schema, so this test
// files a spec and a subtask with it and reads them back through the adapter.
func TestASubtaskCarriesItsSpecAsParent(t *testing.T) {
	if _, err := exec.LookPath("backlog"); err != nil {
		t.Skip("backlog CLI not installed")
	}
	dir := t.TempDir()
	backlog := func(args ...string) {
		t.Helper()
		cmd := exec.Command("backlog", args...)
		cmd.Dir = dir
		cmd.Env = append(os.Environ(), "BACKLOG_CWD="+dir)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("backlog %v: %v (%s)", args, err, out)
		}
	}
	if out, err := exec.Command("git", "-C", dir, "init", "-q").CombinedOutput(); err != nil {
		t.Fatalf("git init: %v (%s)", err, out)
	}
	backlog("init", "parent-test", "--defaults")
	backlog("task", "create", "the spec")
	backlog("task", "create", "a ticket", "--parent", "TASK-1")

	tasks, err := New(dir, DefaultVocabulary()).List()
	if err != nil {
		t.Fatal(err)
	}
	parents := map[string]string{}
	for _, it := range tasks {
		parents[it.ID] = it.Parent
	}
	if p, ok := parents["TASK-1"]; !ok || p != "" {
		t.Fatalf("the spec is top-level, got parents %v", parents)
	}
	if parents["TASK-1.1"] != "TASK-1" {
		t.Fatalf("the ticket should name TASK-1 as its parent, got parents %v", parents)
	}
}
