package daemon

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/DnzzL/herdr-docket/internal/fleet"
	"github.com/DnzzL/herdr-docket/internal/runner"
)

// ownStateDir gives a test a heartbeat of its own: the package shares one
// temp state dir, and another test's tick would read as this one's.
func ownStateDir(t *testing.T) { t.Setenv("HERDR_PLUGIN_STATE_DIR", t.TempDir()) }

func TestLivenessNeverStarted(t *testing.T) {
	ownStateDir(t)
	if got := Liveness(time.Now()); got != "daemon never started" {
		t.Errorf("Liveness = %q, want daemon never started", got)
	}
}

func TestLivenessFreshHeartbeatIsLive(t *testing.T) {
	ownStateDir(t)
	at := time.Date(2026, 10, 28, 12, 0, 0, 0, time.UTC)
	if err := beat(at); err != nil {
		t.Fatal(err)
	}
	if got := Liveness(at.Add(3 * time.Second)); got != "daemon live · tick 3s ago" {
		t.Errorf("Liveness = %q, want daemon live · tick 3s ago", got)
	}
}

func TestLivenessOldHeartbeatIsDown(t *testing.T) {
	ownStateDir(t)
	at := time.Date(2026, 10, 28, 12, 0, 0, 0, time.UTC)
	if err := beat(at); err != nil {
		t.Fatal(err)
	}
	if got := Liveness(at.Add(4 * tickInterval)); got != "daemon down for 1 min" {
		t.Errorf("Liveness = %q, want daemon down for 1 min", got)
	}
}

func TestATickWritesTheHeartbeat(t *testing.T) {
	ownStateDir(t)
	withFleet(t, nil, newMemSource())
	evaluate(runner.New(fakeHost{}, fleet.Settings{Dir: "/fleet"}), fakeHost{}, map[string]bool{})
	if got := Liveness(time.Now()); !strings.HasPrefix(got, "daemon live") {
		t.Errorf("Liveness after a tick = %q, want daemon live …", got)
	}
}

// A heartbeat file that does not hold a time is not a daemon that never
// started: the file was written, so the line says it cannot read it.
func TestLivenessUnreadableHeartbeatSaysSo(t *testing.T) {
	ownStateDir(t)
	if err := os.MkdirAll(filepath.Dir(heartbeatPath()), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(heartbeatPath(), []byte("garbage\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := Liveness(time.Now()); got != "daemon unknown · heartbeat unreadable" {
		t.Errorf("Liveness = %q, want daemon unknown · heartbeat unreadable", got)
	}
}
