package runner

// The settle pass against the real queue, run by hand like the host's live
// probe (TASK-70): the fakes cannot prove that the backlog adapter's verdict
// write and the forge's real pull-request states line up on the tasks the
// board is actually holding.
//
//	HERDR_DOCKET_TESTENV=real LIVE_SETTLE=1 go test ./internal/runner/ -run TestLiveSettlePass -v
//
// HERDR_DOCKET_TESTENV=real is what makes the run see the owner's real
// fleet.yaml: without it internal/testenv sends HOME and the plugin dirs to a
// temp dir, and the pass would list a queue that does not exist.
//
// Not a CI test: it writes the pass's notes and verdicts to the real queue,
// and only for the docket queue — the other queues settle on the daemon's
// own first pass once this lands.

import (
	"os"
	"testing"

	"github.com/DnzzL/herdr-docket/internal/fleet"
	"github.com/DnzzL/herdr-docket/internal/host"
	"github.com/DnzzL/herdr-docket/internal/work"
)

func TestLiveSettlePass(t *testing.T) {
	if os.Getenv("LIVE_SETTLE") == "" {
		t.Skip("set LIVE_SETTLE=1 to settle the real queue")
	}
	settings, err := fleet.LoadSettings()
	if err != nil {
		t.Fatalf("fleet.yaml: %v", err)
	}
	src, err := fleet.NewSource(settings)
	if err != nil {
		t.Fatalf("queue: %v", err)
	}
	tasks, err := src.List()
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	var held []work.Task
	for _, tk := range tasks {
		if work.SourceOf(tk.ID) == "docket" && settles(tk) {
			held = append(held, tk)
		}
	}
	if len(held) == 0 {
		t.Log("nothing held in docket: nothing to settle")
		return
	}
	for _, tk := range held {
		t.Logf("before: %-14s %s", tk.ID, tk.Phase)
	}
	r := New(host.New(), settings)
	r.Settle(src, held)
	after, err := src.List()
	if err != nil {
		t.Fatalf("re-list: %v", err)
	}
	for _, tk := range after {
		for _, h := range held {
			if tk.ID == h.ID {
				t.Logf("after:  %-14s %s", tk.ID, tk.Phase)
			}
		}
	}
}
