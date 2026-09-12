package fleet

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/DnzzL/herdr-fleet/internal/work/backlogmd"
)

// The Backlog.md CLI refuses any status the project config does not declare,
// so a status the adapter can write and the config does not list is a run that
// dies at the first phase change. The two lists are one list.
func TestPatchStatusesWritesEveryStatusTheAdapterUses(t *testing.T) {
	path := configWith(t, "project_name: fleet\nstatuses: [\"To Do\", \"Done\"]\n")
	if err := patchStatuses(path); err != nil {
		t.Fatal(err)
	}
	got := read(t, path)
	for _, status := range backlogmd.Statuses {
		if !strings.Contains(got, `"`+status+`"`) {
			t.Errorf("config is missing %q the adapter can write:\n%s", status, got)
		}
	}
	for _, gone := range []string{`statuses: ["To Do", "Done"]`} {
		if strings.Contains(got, gone) {
			t.Errorf("the old line survived:\n%s", got)
		}
	}
	if !strings.Contains(got, "project_name: fleet") {
		t.Errorf("patching ate the rest of the config:\n%s", got)
	}
}

// Re-running init is normal — an existing fleet is left alone.
func TestPatchStatusesIsIdempotent(t *testing.T) {
	path := configWith(t, "statuses: [\"To Do\"]\n")
	if err := patchStatuses(path); err != nil {
		t.Fatal(err)
	}
	once := read(t, path)
	if err := patchStatuses(path); err != nil {
		t.Fatal(err)
	}
	if twice := read(t, path); twice != once {
		t.Fatalf("second pass changed the file:\n%s\n%s", once, twice)
	}
}

// A config we cannot patch is a config we say so about, rather than one we
// silently leave unable to run anything.
func TestPatchStatusesRefusesAConfigItCannotPatch(t *testing.T) {
	path := configWith(t, "project_name: fleet\n")
	if err := patchStatuses(path); err == nil {
		t.Fatal("want an error when there is no statuses line to patch")
	}
}

// init writes a commented FLEET.md the first time it runs, so the shared
// brief's existence is discoverable. Every line is a comment: an untouched
// scaffold must never reach an agent.
func TestInitBriefScaffoldsACommentedExample(t *testing.T) {
	dir := t.TempDir()
	if err := initBrief(dir); err != nil {
		t.Fatal(err)
	}
	got := read(t, filepath.Join(dir, "FLEET.md"))
	if !strings.Contains(got, "FLEET.md") {
		t.Fatalf("the scaffold does not say what the file is:\n%s", got)
	}
	for _, line := range strings.Split(got, "\n") {
		if line = strings.TrimSpace(line); line != "" && !strings.HasPrefix(line, "#") {
			t.Fatalf("scaffold line is not a comment: %q", line)
		}
	}
}

// A human's brief is not init's to overwrite: re-running init is normal and
// must leave what is already there alone.
func TestInitBriefLeavesAnExistingBriefAlone(t *testing.T) {
	dir := t.TempDir()
	mine := "We are Acme.\n"
	path := filepath.Join(dir, "FLEET.md")
	if err := os.WriteFile(path, []byte(mine), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := initBrief(dir); err != nil {
		t.Fatal(err)
	}
	if got := read(t, path); got != mine {
		t.Fatalf("init changed an existing brief:\n%q", got)
	}
}

func configWith(t *testing.T, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config.yml")
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func read(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}
