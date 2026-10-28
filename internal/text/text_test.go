package text

import (
	"testing"

	"github.com/charmbracelet/x/ansi"
)

// A terminal draws a CJK character or an emoji two cells wide: truncating by
// runes overflowed the line.
func TestTruncateCountsCells(t *testing.T) {
	for _, s := range []string{"日本語のタイトルはとても長い", "a 📎 clip 📎 clip 📎", "plain ascii title here"} {
		got := Truncate(s, 9)
		if w := ansi.StringWidth(got); w > 9 {
			t.Errorf("Truncate(%q, 9) = %q, %d cells", s, got, w)
		}
	}
	if got := Truncate("short", 9); got != "short" {
		t.Errorf("a fitting string changed: %q", got)
	}
}
