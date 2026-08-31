// Package text holds the two-line string helpers the CLI and the pane share.
package text

// Truncate shortens s to at most n runes, ellipsis included. Rune-wise:
// slicing bytes would cut an accented title mid-rune.
func Truncate(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n-1]) + "…"
}
