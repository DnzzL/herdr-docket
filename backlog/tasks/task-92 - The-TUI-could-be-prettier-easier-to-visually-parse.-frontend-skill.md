---
id: TASK-92
title: The TUI could be prettier / easier to visually parse. /frontend-skill
status: Question
assignee: []
created_date: '2026-10-28 15:30'
updated_date: '2026-10-28 15:32'
labels: []
dependencies: []
ordinal: 101000
---

## Implementation Notes

<!-- SECTION:NOTES:BEGIN -->
## Round 1
The pane shipped in 593307f and this card is the first look at it: Home is plain text (runs.go:renderPane), Round has only reverse/faint (round.go frame). ADR 0024 says colours are the terminal's own and rejected Herdr's palettes. Today's width tests count runes, which any ANSI colour would break.

1. Which screens does this cover?
   A. both, Home and Round (reco)
   B. Home only
   C. Round only

2. What should "prettier" buy you?
   A. hierarchy: sections, alignment and one accent that separate daemon / Your turn / In flight at a glance (reco)
   B. more decoration: borders, boxes, more glyphs
   C. less: fewer lines, tighter spacing, nothing new

3. Colour, given ADR 0024's "the terminal's own colours"?
   A. amend 0024: a small semantic palette built from the terminal's 16 ANSI colours, no hex (reco)
   B. no palette: layout and reverse/faint only
   C. adopt Herdr's theme palettes
   T. show me the same screen both ways

4. The row marks (? ! ⇡ ✓ ✎ –) are never explained. Do they stay?
   A. keep the set, add a one-line legend where marks appear (reco)
   B. replace with words: question, blocked, held
   C. redesign the mark set

5. "Easier to parse" is not a unit test as written. What proves it?
   A. golden frame tests pinning both screens at 45 and 120 columns, plus your eyes once (reco)
   B. your eyes only, no test
   C. the visual rule goes in an ADR and the tests only check it holds
<!-- SECTION:NOTES:END -->
