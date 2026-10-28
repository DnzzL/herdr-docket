package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
)

// updateGolden rewrites testdata/pane/*.golden: a golden frame only ever
// changes on purpose, with the diff read first (ADR 0025).
var updateGolden = flag.Bool("update", false, "rewrite the pane golden frames")

// withStyle pins the pane's palette for one test: the golden frames cover
// both the coloured and the plain screen whatever the test's own terminal is.
func withStyle(t *testing.T, colored bool) {
	t.Helper()
	old := paneStyle
	paneStyle = newStyleSet(paneRender, colored)
	t.Cleanup(func() { paneStyle = old })
}

// goldenFrame is one pinned screen: Home or Round, at a phone or a desktop
// width, coloured or plain.
func goldenFrame(t *testing.T, round bool, width int) string {
	t.Helper()
	if round {
		m, _ := roundPane(t, width, 40)
		return m.View()
	}
	return paneAt(t, paneFixture(), width, 40).View()
}

func TestGoldenFrames(t *testing.T) {
	for _, screen := range []string{"home", "round"} {
		for _, width := range []int{45, 120} {
			for _, colored := range []bool{true, false} {
				colour := "plain"
				if colored {
					colour = "colour"
				}
				name := fmt.Sprintf("%s-%d-%s", screen, width, colour)
				t.Run(name, func(t *testing.T) {
					withStyle(t, colored)
					got := goldenFrame(t, screen == "round", width)
					for _, line := range strings.Split(got, "\n") {
						if n := ansi.StringWidth(line); n > width {
							t.Errorf("%d cells at %d: %q", n, width, line)
						}
					}
					if colored != strings.Contains(got, "\x1b[") {
						t.Fatalf("coloured %v but escape sequences present %v:\n%q", colored, strings.Contains(got, "\x1b["), got)
					}
					checkGolden(t, filepath.Join("testdata", "pane", name+".golden"), got)
				})
			}
		}
	}
}

func checkGolden(t *testing.T, path, got string) {
	t.Helper()
	if *updateGolden {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(got+"\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("%v (run `go test -run TestGoldenFrames -update` to write it)", err)
	}
	want := strings.TrimSuffix(string(raw), "\n")
	if got == want {
		return
	}
	gl, wl := strings.Split(got, "\n"), strings.Split(want, "\n")
	for i := 0; i < max(len(gl), len(wl)); i++ {
		g, w := "", ""
		if i < len(gl) {
			g = gl[i]
		}
		if i < len(wl) {
			w = wl[i]
		}
		if g != w {
			t.Fatalf("%s line %d:\n got %q\nwant %q", path, i+1, g, w)
		}
	}
	t.Fatalf("%s differs", path)
}

// NO_COLOR and TERM=dumb both turn every role off, so a script or a dumb
// terminal gets today's plain frame.
func TestNoColourDropsEveryStyle(t *testing.T) {
	for name, env := range map[string]map[string]string{
		"NO_COLOR": {"NO_COLOR": "1"},
		"dumb":     {"TERM": "dumb"},
	} {
		t.Run(name, func(t *testing.T) {
			getenv := func(k string) string { return env[k] }
			if colourEnabled(getenv, true) {
				t.Fatalf("%s: colour enabled", name)
			}
			withStyle(t, colourEnabled(getenv, true))
			for _, view := range []string{goldenFrame(t, false, 45), goldenFrame(t, true, 45)} {
				if strings.Contains(view, "\x1b[") {
					t.Errorf("%s: SGR in a plain frame:\n%s", name, view)
				}
			}
		})
	}
}

// A terminal is what turns colour on; a pipe or a test gets plain bytes.
func TestColourNeedsATerminal(t *testing.T) {
	if colourEnabled(func(string) string { return "" }, false) {
		t.Fatal("colour enabled without a terminal")
	}
	if !colourEnabled(func(string) string { return "" }, true) {
		t.Fatal("colour off on a terminal with no switch set")
	}
}
