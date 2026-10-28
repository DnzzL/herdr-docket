package main

import (
	"strings"
)

// keyAct is what a key does, whatever string triggered it: the switch the
// handler runs once keyHint has matched the key.
type keyAct int

const (
	actNone   keyAct = iota // a hint only: "typing" names the keys the text field takes
	actQuit                 // q or ^c leaves the screen
	actBack                 // Esc leaves the Round for Home
	actMove                 // Home's selection, or the Round's question
	actPress                // Home opens the selected row; the Round presses the focused button
	actOption               // the Round's option on the focused question
	actPick                 // a letter picks its option
	actWords                // the Round's plain-words field takes the keys
	actStop                 // x stops the selected run
	actPause                // p pauses or resumes the fleet
	actDone                 // Enter, Tab or Esc end typing
)

// keyHint is one action a screen takes: the key strings its handler accepts,
// the phrase its hint shows, and the hint line the phrase sits on. A screen
// builds its handler gate and its hint from the same slice, so a key cannot
// be handled and unnamed, or named and dead.
type keyHint struct {
	names []string // the tea.KeyMsg.String values the handler takes
	text  string   // how the hint names them, "" when it names none
	act   keyAct
	line  int // 0 is the first hint line; Home uses one line
}

// match is the hint that takes key k, in the list the screen declared.
func match(hints []keyHint, k string) (keyHint, bool) {
	for _, h := range hints {
		for _, n := range h.names {
			if n == k {
				return h, true
			}
		}
	}
	return keyHint{}, false
}

// hintLines renders the declared hints a line at a time, in order, wrapped to
// width: a long hint wraps rather than losing a word.
func hintLines(hints []keyHint, width int) []string {
	count := 1
	for _, h := range hints {
		count = max(count, h.line+1)
	}
	byLine := make([][]string, count)
	for _, h := range hints {
		if h.text != "" {
			byLine[h.line] = append(byLine[h.line], h.text)
		}
	}
	var out []string
	for _, phrases := range byLine {
		if len(phrases) == 0 {
			continue
		}
		out = append(out, wrap(strings.Join(phrases, " · "), width)...)
	}
	return out
}
