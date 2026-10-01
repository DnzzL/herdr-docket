package daemon

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// Tail is the daemon log's CLI reader: the log exists wherever the plugin's
// startup hook redirected the daemon's stderr, and a human should reach its
// tail without remembering that path. These tests pin the two behaviours a
// reader owes — the tail really is the last lines, and a missing log says
// where it looked.

func TestTailKeepsTheLastNLines(t *testing.T) {
	path := filepath.Join(t.TempDir(), "daemon.log")
	body := ""
	for i := 1; i <= 5; i++ {
		body += "line " + strconv.Itoa(i) + "\n"
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := Tail(path, 2)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0] != "line 4" || got[1] != "line 5" {
		t.Fatalf("tail = %q, want the last two lines", got)
	}
}

func TestTailOfAShorterLogIsTheWholeLog(t *testing.T) {
	path := filepath.Join(t.TempDir(), "daemon.log")
	if err := os.WriteFile(path, []byte("only line\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := Tail(path, 100)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0] != "only line" {
		t.Fatalf("tail = %q, want the whole log", got)
	}
}

func TestTailOfAMissingLogSaysWhereItLooked(t *testing.T) {
	_, err := Tail(filepath.Join(t.TempDir(), "daemon.log"), 10)
	if err == nil || !strings.Contains(err.Error(), "daemon.log") {
		t.Fatalf("err = %v, want the path named", err)
	}
}
