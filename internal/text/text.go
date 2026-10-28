// Package text holds the string helpers the CLI and the pane share, including
// how one task reads.
package text

import "github.com/charmbracelet/x/ansi"

// Truncate shortens s to at most n terminal cells, ellipsis included. Cells,
// not runes or bytes: a CJK character or an emoji is two cells wide, and
// slicing bytes would cut an accented title mid-rune.
func Truncate(s string, n int) string {
	if ansi.StringWidth(s) <= n {
		return s
	}
	return ansi.Truncate(s, n, "…")
}
