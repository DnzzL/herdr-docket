package main

import (
	"os"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/term"
	"github.com/muesli/termenv"
)

// paneRender pins the pane to the terminal's sixteen ANSI colours: the style
// names a colour, the terminal decides what it looks like (ADR 0025). It
// never asks the terminal for a palette, and it is the writer bubbletea draws
// to, so the profile is the same on screen and in a golden frame.
var paneRender = newPaneRenderer()

func newPaneRenderer() *lipgloss.Renderer {
	r := lipgloss.NewRenderer(os.Stdout)
	// SetColorProfile, not termenv.WithProfile: NewRenderer's option only
	// reaches the output, while the style path reads the renderer's explicit
	// profile.
	r.SetColorProfile(termenv.ANSI)
	return r
}

// styleSet is the pane's semantic palette: three roles and reverse focus,
// nothing decorative. on is false when the terminal should see plain text —
// NO_COLOR, TERM=dumb, or no terminal at all — and then every role is the
// identity, so the frame is today's bytes without one SGR sequence.
type styleSet struct {
	accentS, attentionS, faintS, focusS lipgloss.Style
	on                                  bool
}

func newStyleSet(r *lipgloss.Renderer, on bool) styleSet {
	return styleSet{
		accentS:    r.NewStyle().Foreground(lipgloss.Color("6")), // cyan: a structure the eye can jump to
		attentionS: r.NewStyle().Foreground(lipgloss.Color("3")), // yellow: something a person must look at
		faintS:     r.NewStyle().Faint(true),                     // dim: present, not asking
		focusS:     r.NewStyle().Reverse(true),                   // the one selected thing
		on:         on,
	}
}

// colourEnabled is decided once, when the pane starts. NO_COLOR takes any
// non-empty value (the convention scripts already set), TERM=dumb is the
// terminal saying it cannot, and an absent terminal (a test, a pipe) gets
// plain text so nothing downstream has to strip SGR.
func colourEnabled(getenv func(string) string, tty bool) bool {
	if getenv("NO_COLOR") != "" || strings.EqualFold(getenv("TERM"), "dumb") {
		return false
	}
	return tty
}

// paneStyle is the process's one palette. Tests swap it to pin a frame.
var paneStyle = newStyleSet(paneRender, colourEnabled(os.Getenv, term.IsTerminal(os.Stdout.Fd())))

func (s styleSet) accent(t string) string    { return s.render(s.accentS, t) }
func (s styleSet) attention(t string) string { return s.render(s.attentionS, t) }
func (s styleSet) faint(t string) string     { return s.render(s.faintS, t) }
func (s styleSet) focus(t string) string     { return s.render(s.focusS, t) }

// off is a disabled control: today's faint, not a fourth colour.
func (s styleSet) off(t string) string { return s.render(s.faintS, t) }

func (s styleSet) render(style lipgloss.Style, t string) string {
	if !s.on || t == "" {
		return t
	}
	return style.Render(t)
}

// paintFirst styles the first plain occurrence of sub in an already wrapped
// and truncated line. A line cut before sub stays unstyled: the style never
// reaches text the screen does not draw.
func paintFirst(s, sub string, style func(string) string) string {
	if sub == "" {
		return s
	}
	i := strings.Index(s, sub)
	if i < 0 {
		return s
	}
	return s[:i] + style(sub) + s[i+len(sub):]
}
