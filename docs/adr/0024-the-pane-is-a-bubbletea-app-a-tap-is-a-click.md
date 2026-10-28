# The pane is a bubbletea app, and a tap is a click

The failure this answers: the owner answers grill rounds from a phone in
Termux and from a desktop terminal. A line prompt makes every answer a typed
letter and Enter, shows no context before the questions, and the raw-ANSI pane
cannot take a click. Every Herdr plugin with a pane people click on (herdr-plus,
sesh, workflows, paddock) is a bubbletea app; Herdr forwards mouse events to a
pane that turns on mouse reporting, and Termux turns a tap into a left click
(modes 1000/1002).

**`herdr-docket pane` is a bubbletea v1.3 + lipgloss v1.1 app**, alt screen,
`WithMouseCellMotion` (1002: Termux has no 1003). Two screens: Home (daemon
line, Your turn, In flight, Settled) and Round (the round's context, its
questions with each option as a button, a free-text field, Send). A click or
tap on a row or a button does what Enter does there; the wheel scrolls; every
key that works today keeps working. Clicks map to rows by hand from the
rendered layout: no zone library. Colours are the terminal's own.
bubbletea v1's package init asks the terminal for its background (OSC 11)
and waits up to 5 s for an answer, in every command the binary runs;
`internal/quietterm` sets the background first, so no command queries the
terminal.

**The pane writes through the CLI's own paths only**: `task answer` and its
lock check for a Round, `stop`'s close of a run's workspace (`x`), and
`pause`/`resume`'s flag (`p`); nothing else. Queue reads and writes run as
bubbletea commands, off the update loop: a GitHub board never freezes the
screen. `herdr-docket questions` opens the Round screen on a
TTY; without one it stays the line prompt, for scripts and tests.

**The Round carries an optional context paragraph** between `## Round N` and
question 1: what the triager found, so a round is answerable without opening
the card.

**Rejected.** *Parsing SGR mouse sequences by hand on the raw-ANSI pane*: a
terminal layer of our own to avoid the dependency every Herdr plugin already
carries. *The line prompt only*: two keystrokes per answer, no tap, no context
screen. *Copying Herdr's theme palettes* (as sesh does): a copy that drifts;
the terminal's colours until something clashes. *bubbletea v2*: the newer API,
but the owner's other plugins are on v1.3; one API to know.

Amends [0008](0008-the-board-is-a-triage-surface.md)'s removal of the board TUI
for the pane only.

**Status:** accepted
