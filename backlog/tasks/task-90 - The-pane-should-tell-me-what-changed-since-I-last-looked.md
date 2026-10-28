---
id: TASK-90
title: The pane should tell me what changed since I last looked
status: Question
assignee: []
created_date: '2026-10-28 15:24'
updated_date: '2026-10-28 15:26'
labels: []
dependencies: []
ordinal: 99000
---

## Implementation Notes

<!-- SECTION:NOTES:BEGIN -->
## Round 1

The card is empty: no spec, and TASK-91 carries the same title one minute later. My reading: `herdr-docket pane`'s Home (`runs.go:630`) shows only *now* — daemon line, Your turn, in flight, last 5 settled. Check in after an hour on the phone and everything that moved in between is gone; the pane never says "new".
Files: runs.go (Home layout, paneData), internal/history/history.go, internal/daemon/heartbeat.go, internal/hostpath (state dir: history.jsonl, heartbeat, paused). Linked: TASK-63 (held merges announced), TASK-64 (daily digest), ADR 0024. `T. try and show me` is allowed on every question.

1. What counts as news?
   A. Only what wants you or ended: a card newly in Question, newly Blocked, a newly held merge, a run that settled (done/failed/cancelled) (reco)
   B. A too, plus every phase move (To Do → In Progress, a delivery)
   C. Runs only — no queue moves

2. When does "last looked" advance?
   A. On a clean quit, in a `last-look` file in the state dir: reopen after a night, see the night (reco)
   B. On the pane's first frame, so it means "since this pane opened"
   C. Only when I press a key to mark it read

3. How much does it show?
   A. Everything since the marker, capped at 8 lines then "+N more" (reco)
   B. Last 24 h only
   C. No cap, it scrolls inside Home

4. Where does it sit, and what shape?
   A. A block at the top under the daemon line — "Since you last looked (3)", one line per event, absent when empty (reco)
   B. One digest line, tap/Enter expands it
   C. No block: a `•` on the rows that changed

5. TASK-91, same title, one minute later, also empty and in Triage.
   A. TASK-90 is the card; you close TASK-91 as a duplicate — I leave it untouched (reco)
   B. They are two separate wants; say what the second adds
   C. Merge them by wording the second as the follow-up it is
<!-- SECTION:NOTES:END -->
