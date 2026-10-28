# The pane styles text in the terminal's own sixteen ANSI colours

The failure this answers: the pane was one style of text. Home drew blank
lines and headings that read like every row, so at 45 columns — the owner's
phone — the daemon, Your turn, In flight and Settled ran together. Round had
reverse for focus and faint for a disabled Send, nothing else. The marks a
person acts on (`? ! ⇡ ✓ ✎ –`) were never explained. ADR 0024 said "colours
are the terminal's own", which is a decision not to have a palette; without
one, the next change would drift.

**Three semantic roles, named by their job, never by their look.** *Accent*
marks structure: Home's section headings and the selected row's mark, Round's
question numbers, its `(reco)` options and a picked `✓`. *Attention* marks
something a person must look at: a stale run's age and a daemon line that is
down. *Faint* is present but not asking: Settled rows, the `nothing` and `no
run` notes, a disabled Send. Focus stays reverse. There is no fourth colour
and nothing decorative.

**The palette is the terminal's sixteen ANSI colours, by name only.**
Accent is cyan (`6`), attention yellow (`3`), faint the dim attribute; the
renderer is pinned to `termenv.ANSI`, so no truecolour value, no hex, no
copied Herdr theme and no OSC 11 query ever reaches the screen. The terminal
decides what cyan and yellow look like on its background, which is what ADR
0024 meant and this ADR keeps. `internal/quietterm` is unchanged.

**Style lands on text already wrapped and truncated.** Width helpers measure
display cells (`ansi.StringWidth`), never runes or bytes, and no SGR sequence
is split by a width helper: a line that is styled is first plain, and a styled
token is a whole word. Home's card ids share one padding column so titles line
up. Each screen gains one legend line where its marks appear — Home `?
question · ! blocked · ⇡ merge held`, Round `✓ picked · ✎ in words · –
unanswered` — wrapped, not cut, at 45 columns. No borders, no boxes, no new
glyph, no new screen; keys, rows, hit boxes and click targets are unchanged.

**`NO_COLOR` (any non-empty value), `TERM=dumb`, or no terminal at all turns
every role off**, including reverse and faint: the frame is plain bytes and a
CLI path or a test that reads output needs no stripping. `NO_COLOR=1 go test
./...` is green.

**Golden frames pin the decision.** `testdata/pane/` holds Home and Round at
45 and 120 columns, coloured and plain, on the fixed fixture time; `go test
-run TestGoldenFrames -update` rewrites them, and a diff otherwise fails the
test. The pane's own terminal decides nothing in those tests: the profile is
pinned, so a maintainer's terminal cannot change a golden.

**Rejected.** *Hex values or a copied theme palette*: a colour that assumes a
background, and a copy that drifts from its source. *Only reverse and faint*:
no way to separate a stale run from a fresh one, or a heading from a row.
*Borders and boxes*: width spent on decoration at 45 columns, and a second
layout to keep in step with the hit boxes. *Styling before wrapping*: a width
helper counts SGR bytes and cuts a sequence, which is the bug this ordering
exists to prevent.

Amends [0024](0024-the-pane-is-a-bubbletea-app-a-tap-is-a-click.md)'s
"Colours are the terminal's own": the decision stands, this names what it
means and the roles it is spent on.

**Status:** accepted
