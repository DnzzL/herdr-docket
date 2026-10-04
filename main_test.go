package main

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/DnzzL/herdr-docket/internal/changelog"
	"github.com/DnzzL/herdr-docket/internal/fleet"
	"github.com/DnzzL/herdr-docket/internal/history"
	"github.com/DnzzL/herdr-docket/internal/hostpath"
	"github.com/DnzzL/herdr-docket/internal/work"
)

// `herdr-docket run` is explicit human intent: it must reach a parked agent
// anyway, so pausing parks the scheduler rather than forbidding the work.
func TestManualRunReachesADisabledAgent(t *testing.T) {
	agents := map[string]fleet.Agent{"dev": {Name: "dev", Disabled: true}}
	task := work.Task{ID: "TASK-9", Assignee: "dev", Open: true}

	agent, err := routedAgent(agents, task, defaults(""))
	if err != nil {
		t.Fatalf("manual run must bypass the pause: %v", err)
	}
	if agent.Name != "dev" || !agent.Disabled {
		t.Fatalf("want the parked dev, got %+v", agent)
	}
}

func TestManualRunStillRejectsAnUnknownAgent(t *testing.T) {
	agents := map[string]fleet.Agent{"dev": {Name: "dev"}}
	if _, err := routedAgent(agents, work.Task{ID: "T", Assignee: "ghost"}, defaults("")); err == nil {
		t.Fatal("an unknown assignee must still error")
	}
}

// The default-agent route resolves the same way, parked or not.
func TestManualRunResolvesTheDefaultAgent(t *testing.T) {
	agents := map[string]fleet.Agent{"dev": {Name: "dev", Disabled: true}}
	agent, err := routedAgent(agents, work.Task{ID: "T"}, defaults("dev"))
	if err != nil || agent.Name != "dev" {
		t.Fatalf("default agent should route: %v %+v", err, agent)
	}
}

// The history line answers both audit questions on one row: how long the run
// took and how it ended, when the closing record carries them.
func TestHistoryLineShowsDurationAndVerdict(t *testing.T) {
	r := history.Record{
		At:              time.Date(2026, 9, 1, 10, 0, 0, 0, time.UTC),
		Status:          history.StatusDone,
		Task:            "TASK-1",
		Trigger:         "poll",
		DurationSeconds: 90,
		Verdict:         "done",
	}
	line := formatHistory(r)
	for _, want := range []string{"1m30s", "done", "TASK-1", "poll"} {
		if !strings.Contains(line, want) {
			t.Fatalf("history line %q missing %q", line, want)
		}
	}
}

// A run still in flight has no closing record, so its line shows neither a
// duration nor a verdict — just the mechanics.
func TestHistoryLineOmitsWhatTheRunHasNotReported(t *testing.T) {
	r := history.Record{At: time.Now(), Status: history.StatusRunning, Task: "TASK-1", Trigger: "poll"}
	line := formatHistory(r)
	if strings.Contains(line, "0s") || strings.Contains(line, "done") {
		t.Fatalf("an open run must not invent duration or verdict: %q", line)
	}
}

// defaults is the routing default as the settings express it — one name for
// the whole fleet, which is what these cases are about.
func defaults(name string) fleet.Defaults {
	return fleet.Settings{DefaultAgent: name}.Defaults()
}

// `auth` takes a queue and, for a queue that wants a token handed to it, one
// flag. What it does *with* them is fleet's business; that it insists on a
// queue at all, and says which queues exist, is the CLI's.
func TestAuthWantsAQueueToSignInTo(t *testing.T) {
	for _, tc := range []struct {
		name string
		args []string
		want string
	}{
		{"nothing at all", nil, "usage:"},
		{"a token and no queue", []string{"--token", "ghp_x"}, "usage:"},
		{"a queue and then another", []string{"github", "basecamp"}, "unexpected"},
		{"a token with nothing after it", []string{"--token"}, "usage:"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			err := authCmd(tc.args)
			if err == nil {
				t.Fatal("want an error")
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("error %q does not mention %q", err, tc.want)
			}
			if !strings.Contains(err.Error(), "github") {
				t.Errorf("error %q must name the queues that can be signed in to", err)
			}
		})
	}
}

// The history line carries the delivery beside the verdict: which branch the
// run produced, how far it moved, the pull request it went out as, and — the
// one fact the fleet checks itself — that a worktree was about to be destroyed
// holding uncommitted work.
func TestHistoryLineShowsTheDelivery(t *testing.T) {
	r := history.Record{
		At:              time.Date(2026, 9, 1, 10, 0, 0, 0, time.UTC),
		Status:          history.StatusDone,
		Task:            "TASK-1",
		Trigger:         "poll",
		Verdict:         "done",
		Branch:          "fleet/a-1",
		Commits:         3,
		PullRequest:     "https://example.com/pr/5",
		Uncommitted:     true,
		DurationSeconds: 12,
	}
	line := formatHistory(r)
	for _, want := range []string{"fleet/a-1", "+3", "https://example.com/pr/5", "uncommitted"} {
		if !strings.Contains(line, want) {
			t.Fatalf("history line %q missing %q", line, want)
		}
	}
}

// A verifier's verdict rides on the closing record since ADR 0013, but a
// supervising human reads history, not history.jsonl: without it here the
// queue's PASS/FAIL is invisible from the CLI.
func TestHistoryLineShowsTheVerification(t *testing.T) {
	r := history.Record{
		At:           time.Date(2026, 9, 1, 10, 0, 0, 0, time.UTC),
		Status:       history.StatusDone,
		Task:         "TASK-1",
		Trigger:      "poll",
		Verdict:      "done",
		Verification: "PASS",
		PatchID:      "9ad5c1e2c0f4b8a76d3e5f7a9c1b2d4e5f6a7b8c",
	}
	line := formatHistory(r)
	for _, want := range []string{"verified PASS", "p=9ad5c1e2"} {
		if !strings.Contains(line, want) {
			t.Fatalf("history line %q missing %q", line, want)
		}
	}
}

// The one-step factory setup is the only flag init takes: an unknown
// argument is refused before anything on disk is touched, and the refusal
// names the flag so the discovery path is the error message.
func TestInitRefusesAnUnknownFlagBeforeTouchingAnything(t *testing.T) {
	if err := initCmd("--frobnicate"); err == nil {
		t.Fatal("want an error")
	} else if !strings.Contains(err.Error(), "--factory") {
		t.Errorf("error must point at the flag that exists: %v", err)
	}
}

// The daemon log is the one log the fleet writes on its own behalf, and a
// human should reach its tail from the CLI without remembering the state dir.
// The command reads what is on disk: it is a report, not a second log.
func TestLogsPrintsTheDaemonLogTail(t *testing.T) {
	t.Setenv("HERDR_PLUGIN_STATE_DIR", t.TempDir())
	if err := os.WriteFile(filepath.Join(hostpath.StateDir(), "daemon.log"),
		[]byte("first\nmiddle\nlast\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	if err := logsCmd([]string{"-n", "2"}, &buf); err != nil {
		t.Fatal(err)
	}
	if got := buf.String(); got != "middle\nlast\n" {
		t.Fatalf("logs -n 2 = %q, want the last two lines", got)
	}
}

// A missing log is an answer, not an empty screen: where it looked, and why
// there is nothing there.
func TestLogsNamesTheLogItCouldNotFind(t *testing.T) {
	t.Setenv("HERDR_PLUGIN_STATE_DIR", t.TempDir())
	err := logsCmd(nil, &bytes.Buffer{})
	if err == nil || !strings.Contains(err.Error(), "daemon.log") {
		t.Fatalf("err = %v, want the log's path named", err)
	}
}

// TASK-45: the cut commit is on the line, so a PR that carries more than the
// run branched from can be seen from history alone.
func TestHistoryLineNamesTheCommitTheRunCutFrom(t *testing.T) {
	r := history.Record{At: time.Now(), Status: history.StatusRunning, Task: "TASK-1", Branch: "fleet/a-1", BaseCommit: "c0074fe"}
	line := formatHistory(r)
	if !strings.Contains(line, "cut c0074fe") {
		t.Fatalf("history line %q missing the cut commit", line)
	}
	r.BaseCommit = ""
	if strings.Contains(formatHistory(r), "cut ") {
		t.Fatal("a run with no base commit on file must not invent one")
	}
}

// TASK-62: history.jsonl marks the timeout, but a supervising human reads
// the CLI — a delivery that arrived past its deadline must be visible there,
// on the closing record the reader shows.
func TestHistoryLineMarksATimedOutRun(t *testing.T) {
	r := history.Record{
		At: time.Now(), Status: history.StatusDone, Task: "TASK-1", Trigger: "poll",
		DurationSeconds: 90, Verdict: "done", TimedOut: true,
	}
	if line := formatHistory(r); !strings.Contains(line, "timed-out") {
		t.Fatalf("history line %q missing the timed-out mark", line)
	}
	r.TimedOut = false
	if line := formatHistory(r); strings.Contains(line, "timed-out") {
		t.Fatalf("an on-time run must not be marked late: %q", line)
	}
}

// changelogRepo lays out a repo root with a changelog.d/ and a CHANGELOG.md
// and makes it the working directory, the way the command is run.
func changelogRepo(t *testing.T, frags map[string]string) {
	t.Helper()
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, changelog.Dir), 0o755); err != nil {
		t.Fatal(err)
	}
	for name, body := range frags {
		if err := os.WriteFile(filepath.Join(root, changelog.Dir, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	const existing = "# Changelog\n\nWhat changed.\n\n## v0.8.0 — 2026-10-02\n\n- **Shipped.**\n"
	if err := os.WriteFile(filepath.Join(root, "CHANGELOG.md"), []byte(existing), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Chdir(root)
}

func TestChangelogPreviewsWhatTheNextReleaseWillSayAndWritesNothing(t *testing.T) {
	changelogRepo(t, map[string]string{"TASK-62.md": "- **An entry.**"})
	var out strings.Builder
	if err := changelogCmd(nil, &out); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "An entry") {
		t.Errorf("preview lost the entry: %q", out.String())
	}
	raw, _ := os.ReadFile("CHANGELOG.md")
	if strings.Contains(string(raw), "An entry") {
		t.Error("a preview must not write to CHANGELOG.md")
	}
	if _, err := os.Stat(filepath.Join(changelog.Dir, "TASK-62.md")); err != nil {
		t.Error("a preview must not consume the fragments")
	}
}

func TestChangelogSaysSoWhenNothingIsPending(t *testing.T) {
	changelogRepo(t, nil)
	var out strings.Builder
	if err := changelogCmd(nil, &out); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "nothing pending") {
		t.Errorf("want a plain answer, got %q", out.String())
	}
}

func TestChangelogReleaseCutsTheSectionAndConsumesTheFragments(t *testing.T) {
	changelogRepo(t, map[string]string{"TASK-62.md": "- **An entry.**"})
	var out strings.Builder
	if err := changelogCmd([]string{"release", "v0.9.0"}, &out); err != nil {
		t.Fatal(err)
	}
	raw, _ := os.ReadFile("CHANGELOG.md")
	if !strings.Contains(string(raw), "An entry") || !strings.Contains(string(raw), "## v0.9.0 — ") {
		t.Errorf("the section did not land:\n%s", raw)
	}
	if _, err := os.Stat(filepath.Join(changelog.Dir, "TASK-62.md")); !os.IsNotExist(err) {
		t.Error("the released fragment should be gone")
	}
}

func TestChangelogRefusesAnArgumentItDoesNotKnow(t *testing.T) {
	changelogRepo(t, map[string]string{"TASK-62.md": "- **An entry.**"})
	for _, args := range [][]string{{"cut", "v0.9.0"}, {"release"}, {"release", "v0.9.0", "extra"}} {
		if err := changelogCmd(args, io.Discard); err == nil {
			t.Errorf("%v was accepted", args)
		}
	}
	if _, err := os.Stat(filepath.Join(changelog.Dir, "TASK-62.md")); err != nil {
		t.Error("a refused command must leave the fragments alone")
	}
}
