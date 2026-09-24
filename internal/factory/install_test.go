package factory

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// fakeHerdr writes an executable that answers `plugin config-dir` the way the
// real CLI does — with the given path, or not at all — so the tests script
// the one external call Install makes.
func fakeHerdr(t *testing.T, answer string, exit int) string {
	t.Helper()
	dir := t.TempDir()
	script := filepath.Join(dir, "herdr")
	body := "#!/bin/sh\n"
	if answer != "" {
		body += "echo '" + answer + "'\n"
	}
	body += "exit " + itoa(exit) + "\n"
	if err := os.WriteFile(script, []byte(body), 0o755); err != nil {
		t.Fatal(err)
	}
	return script
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b []byte
	for n > 0 {
		b = append([]byte{byte('0' + n%10)}, b...)
		n /= 10
	}
	return string(b)
}

func TestInstallWritesTheFleetSidePersonas(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HERDR_BIN_PATH", fakeHerdr(t, "", 1)) // no automations plugin

	var out bytes.Buffer
	if err := Install(dir, &out); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"intake", "stall", "lookback"} {
		p := filepath.Join(dir, "agents", name, "AGENT.md")
		body, err := os.ReadFile(p)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if !strings.Contains(string(body), "You are this fleet's "+namePrefix(name)) {
			t.Errorf("%s persona does not open like its example:\n%s", name, firstLine(string(body)))
		}
		// The workdir is rewritten to the fleet it was installed into — the
		// example's ~/fleet is a default, not a truth about this fleet.
		if strings.Contains(string(body), "workdir: ~/fleet") {
			t.Errorf("%s still claims ~/fleet; this fleet lives in %s", name, dir)
		}
		if !strings.Contains(string(body), "workdir: "+dir) {
			t.Errorf("%s workdir not pointed at the fleet dir", name)
		}
	}
	if !strings.Contains(out.String(), "intake") || !strings.Contains(out.String(), "factory.md") {
		t.Errorf("output must say what it wrote and where to read next:\n%s", out.String())
	}
	if !strings.Contains(out.String(), "Report only") {
		t.Errorf("output must point at the dry-run dial before the schedule is trusted:\n%s", out.String())
	}
}

func namePrefix(name string) string {
	switch name {
	case "intake":
		return "intake"
	case "stall":
		return "stall sweep"
	default:
		return "lookback"
	}
}

func firstLine(s string) string {
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		return s[:i]
	}
	return s
}

// A persona the fleet already has is never rewritten: the human's edits are
// the point of these files, and an upgrade that ate them would be the fleet
// knowing better.
func TestInstallNeverOverwritesAnExistingPersona(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "agents", "stall", "AGENT.md")
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte("the human's own sweep\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("HERDR_BIN_PATH", fakeHerdr(t, "", 1))

	var out bytes.Buffer
	if err := Install(dir, &out); err != nil {
		t.Fatal(err)
	}
	body, _ := os.ReadFile(p)
	if string(body) != "the human's own sweep\n" {
		t.Errorf("existing persona was rewritten: %q", body)
	}
	if !strings.Contains(out.String(), "skipped") {
		t.Errorf("output must say what it skipped:\n%s", out.String())
	}
}

// Without the automations plugin the personas still land, and the output says
// the schedules are still owed — a note, never a failure.
func TestInstallSaysTheSchedulesAreOwedWhenAutomationsIsAbsent(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HERDR_BIN_PATH", fakeHerdr(t, "", 1))

	var out bytes.Buffer
	if err := Install(dir, &out); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "await schedules") {
		t.Errorf("output must say the personas await schedules:\n%s", out.String())
	}
}

// With the plugin present, the driving entries are inserted after the
// automations key, and a second run changes nothing.
func TestInstallAppendsTheLoopEntriesAndIsIdempotent(t *testing.T) {
	dir := t.TempDir()
	cfg := t.TempDir()
	t.Setenv("HERDR_BIN_PATH", fakeHerdr(t, cfg, 0))

	existing := "automations:\n  - name: nightly\n    cron: \"0 3 * * *\"\n    repo: ~/Projects/myapp\n    prompt: |\n      hello\n"
	path := filepath.Join(cfg, "automations.yaml")
	if err := os.WriteFile(path, []byte(existing), 0o644); err != nil {
		t.Fatal(err)
	}

	var out bytes.Buffer
	if err := Install(dir, &out); err != nil {
		t.Fatal(err)
	}
	raw, _ := os.ReadFile(path)
	got := string(raw)
	if !strings.HasPrefix(got, "automations:\n") || !strings.Contains(got, "- name: nightly") {
		t.Fatalf("existing entries damaged:\n%s", got)
	}
	keyAt, entriesAt := strings.Index(got, "automations:"), strings.Index(got, "- name: intake")
	if keyAt < 0 || entriesAt < keyAt {
		t.Errorf("the loop must land inside the automations list, not above the key:\n%s", got)
	}
	if !strings.Contains(got, "repo: "+dir) {
		t.Errorf("entries must point at the fleet they were installed into:\n%s", got)
	}
	if !strings.Contains(got, "Report only") {
		t.Errorf("the intake entry must keep its dry-run dial visible:\n%s", got)
	}

	// Idempotent: the same command again writes nothing new.
	before := got
	if err := Install(dir, &bytes.Buffer{}); err != nil {
		t.Fatal(err)
	}
	raw, _ = os.ReadFile(path)
	if string(raw) != before {
		t.Errorf("second run changed the file:\nbefore:\n%s\nafter:\n%s", before, raw)
	}
}

// A plugin present but never configured gets a fresh file — the loop's four
// entries in a valid file, not an error.
func TestInstallCreatesTheAutomationsFileWhenThePluginHasNone(t *testing.T) {
	dir := t.TempDir()
	cfg := t.TempDir()
	t.Setenv("HERDR_BIN_PATH", fakeHerdr(t, cfg, 0))

	var out bytes.Buffer
	if err := Install(dir, &out); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(filepath.Join(cfg, "automations.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(string(raw), "automations:\n") || !strings.Contains(string(raw), "- name: lookback") {
		t.Fatalf("created file:\n%s", raw)
	}
}

// The fleet's own config is not this command's business: whatever sources and
// statuses the fleet declares, they are still there afterwards.
func TestInstallLeavesTheFleetsOwnConfigAlone(t *testing.T) {
	dir := t.TempDir()
	fleetYAML := "sources:\n  myapp:\n    dir: ~/Projects/myapp\n    statuses:\n      todo: ready for agent\n"
	path := filepath.Join(dir, "fleet.yaml")
	if err := os.WriteFile(path, []byte(fleetYAML), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("HERDR_BIN_PATH", fakeHerdr(t, "", 1))

	if err := Install(dir, &bytes.Buffer{}); err != nil {
		t.Fatal(err)
	}
	raw, _ := os.ReadFile(path)
	if string(raw) != fleetYAML {
		t.Errorf("fleet.yaml changed:\n%s", raw)
	}
}
