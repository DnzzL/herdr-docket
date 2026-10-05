package factory

import (
	"bytes"
	"os"
	"path/filepath"
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

	for _, r := range roles {
		if !strings.Contains(string(examples), r.body) {
			t.Errorf("role %s is not in docs/examples.md verbatim — one of the two has drifted", r.name)
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

// The merge digest is the one entry that does its own work instead of filing
// a task, and that exception is only safe spelled out. What init writes must
// carry the entry — daily, naming the label it reads, the issue it keeps, and
// every fact each digest line promises — and docs/factory.md must carry the
// sentence that licenses a schedule doing its own work at all.
func TestTheMergeDigestIsShippedDailyAndLicensedInDocs(t *testing.T) {
	dir := t.TempDir()
	cfg := t.TempDir()
	t.Setenv("HERDR_BIN_PATH", fakeHerdr(t, cfg, 0))
	if err := Install(dir, &bytes.Buffer{}); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(filepath.Join(cfg, "automations.yaml"))
	if err != nil {
		t.Fatal(err)
	}

	block := entryBlock(string(raw), "merge-digest")
	if block == "" {
		t.Fatal("init writes no merge-digest entry — the daily digest was never scheduled")
	}
	cron := ""
	for _, line := range strings.Split(block, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "cron:") {
			cron = strings.Trim(strings.TrimSpace(strings.TrimPrefix(line, "cron:")), `"`)
		}
	}
	fields := strings.Fields(cron)
	if len(fields) != 5 || fields[2] != "*" || fields[3] != "*" || fields[4] != "*" {
		t.Errorf("merge-digest must run once a day; its cron is %q", cron)
	}
	for _, promised := range []string{"merge-needed", "Merges waiting", "task id", "reason", "waiting since", "gh issue close"} {
		if !strings.Contains(block, promised) {
			t.Errorf("the merge-digest entry never names %q — the issue could not carry it", promised)
		}
	}

	factory, err := os.ReadFile("../../docs/factory.md")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(factory), "the digest decides nothing — it only lists") {
		t.Error("docs/factory.md never says why a schedule may do its own work: the digest decides nothing — it only lists")
	}
}
