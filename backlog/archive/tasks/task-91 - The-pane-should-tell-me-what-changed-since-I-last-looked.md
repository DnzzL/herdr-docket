---
id: TASK-91
title: The pane should tell me what changed since I last looked
status: Done
assignee: []
created_date: '2026-10-28 15:25'
updated_date: '2026-10-28 15:40'
labels: []
dependencies: []
ordinal: 100000
---

## Implementation Notes

<!-- SECTION:NOTES:BEGIN -->
## Round 1

Card is an empty title: "The pane should tell me what changed since I last looked". Today `herdr-docket pane` (runs.go, renderPane) draws four things: the daemon line, Your turn (Question rounds, Blocked cards, merges held for you), runs in flight, and the last 5 settled runs. Nothing persists between pane sessions — there is no "last looked" anywhere in the tree, so this needs one. Files: runs.go (rendering, paneModel), main_pane_test.go; state can live in the plugin state dir (hostpath.StateDir, as history and pause already do). Linked: TASK-63 (merge popups), TASK-64 (daily digest) — both push at you; this is the pull side.
Assumed unless you say otherwise: one marker for all queues, surviving a pane restart.

1. What counts as "changed"?
   A. what needs me plus what finished: a Question round, a Blocked card, a held merge, a run that settled (reco)
   B. all of A, plus any queue edit — a new card, a phase move, a reassignment, an edited body
   C. only the Your turn list; settled runs stay unmarked
   T. try and show me

2. What decides a line is new?
   A. a persisted timestamp: it moves when the pane quits, so a restart marks everything since then (reco)
   B. per-item: a card or run stays marked until you open it (a Round) or it leaves Your turn; seen ids are kept in a state file
   C. an explicit key/tap marks all seen; quitting the pane does not count as looking
   T. try and show me

3. What does the mark look like on screen?
   A. a `●` on each new row, plus one header line: `3 new since 14:32` (reco)
   B. a divider line — `— since you last looked —` — between the old rows and the new ones, rows unchanged
   C. only a count in the daemon line; rows stay as they are
   T. try and show me

4. A run or card changed while you were away but no longer fits the frame (Settled shows 5)?
   A. extend Settled to everything since your last look, capped at 20 with a final `+N more` (reco)
   B. mark only what the frame already draws; an older unseen run leaves no trace
   C. no cap — list everything since your last look
   T. try and show me

5. Should the unseen count reach outside the pane?
   A. no — the pane only; popups keep carrying failures and merges (reco)
   B. yes — the Herdr tab title carries the count
   T. try and show me

Duplicate of TASK-90 (same title, filed a minute later).
<!-- SECTION:NOTES:END -->
