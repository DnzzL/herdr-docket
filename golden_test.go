package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/DnzzL/herdr-docket/internal/work"
)

var updateGolden = flag.Bool("update", false, "rewrite testdata/pane golden frames")

// golden pins one screen's bytes: the frames are the contract the owner sees
// on the phone, so a hint or a key that moves shows up as a diff.
func golden(t *testing.T, name, got string) {
	t.Helper()
	path := filepath.Join("testdata", "pane", name+".golden")
	if *updateGolden {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(got), 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("%v (run go test -run Golden -update)", err)
	}
	if got != string(want) {
		t.Errorf("%s drifted; run go test -run Golden -update and read the diff\n--- got ---\n%s", name, got)
	}
}

func TestGoldenFramesPinHomeAndRoundAt45And120Columns(t *testing.T) {
	for _, width := range []int{45, 120} {
		m := paneAt(t, paneFixture(), width, 40)
		golden(t, fmt.Sprintf("home-%d", width), m.View())

		r, _ := roundPane(t, width, 40)
		golden(t, fmt.Sprintf("round-%d", width), r.View())
	}
}

// A Home taller than the pane keeps its hint in the scroll: pinned at the
// bottom so it cannot fall below the View window again.
func TestGoldenTallHomeShowsTheHintAtTheBottom(t *testing.T) {
	d := paneFixture()
	for i := 0; i < 16; i++ {
		d.Queue = append(d.Queue, work.Task{ID: fmt.Sprintf("TASK-%d", 100+i), Title: "A question", Question: true, Phase: string(work.PhaseQuestion)})
	}
	m := paneAt(t, d, 45, 12)
	for i := 0; i < 200 && !strings.Contains(m.View(), "q/^c quit"); i++ {
		next, _ := m.Update(tea.MouseMsg{Action: tea.MouseActionPress, Button: tea.MouseButtonWheelDown})
		m = next.(paneModel)
	}
	golden(t, "home-tall-45", m.View())
}

// No hint line, on any screen or state, is wider than the pane: a hint wraps
// with the same helper the rest of the screen does.
func TestNoHintLineExceedsThePaneAt45Or120Columns(t *testing.T) {
	for _, width := range []int{45, 120} {
		views := map[string]string{}
		views["home"] = paneAt(t, paneFixture(), width, 40).View()
		round, _ := roundPane(t, width, 40)
		views["round"] = round.View()
		typing := send(round, tea.KeyMsg{Type: tea.KeyTab})
		views["typing"] = typing.View()
		for name, view := range views {
			for _, line := range strings.Split(view, "\n") {
				if n := lipgloss.Width(line); n > width {
					t.Errorf("%s at %d: %d cells: %q", name, width, n, line)
				}
			}
		}
	}
}
