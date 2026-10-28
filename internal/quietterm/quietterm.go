// Package quietterm keeps every herdr-docket command from querying the
// terminal. bubbletea v1's package init calls lipgloss.HasDarkBackground,
// which sends OSC 11 and waits up to 5 s for a terminal that never answers:
// a tax on every `task note` an agent runs from a pane. Setting the
// background explicitly first makes that call a no-op.
//
// It works by init order, which the Go spec fixes (since 1.21): packages
// initialize in import-path order once their imports are done, and
// "github.com/DnzzL/…" sorts before "github.com/charmbracelet/bubbletea", so
// this init runs right after lipgloss's and before bubbletea's. It must
// import nothing but lipgloss, and main must import it (blank).
// TestACommandOnASilentTerminalSendsNoQueryAndReturnsAtOnce guards it.
//
// The pane draws with the terminal's own colours (reverse, faint), so the
// background it assumes changes nothing on screen.
package quietterm

import "github.com/charmbracelet/lipgloss"

func init() { lipgloss.SetHasDarkBackground(true) }
