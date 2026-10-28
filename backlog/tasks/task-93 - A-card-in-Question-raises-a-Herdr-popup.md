---
id: TASK-93
title: A card in Question raises a Herdr popup
status: Question
assignee: []
created_date: '2026-10-28 15:32'
updated_date: '2026-10-28 15:34'
labels: []
dependencies: []
ordinal: 102000
---

## Description

<!-- SECTION:DESCRIPTION:BEGIN -->
When a card moves to Question, the daemon raises the same Herdr popup a merge the owner owes raises (ADR 0015): card id, title, round number. It never opens the pane itself. Unknown to check: whether a Herdr notification can open a plugin pane on click.
<!-- SECTION:DESCRIPTION:END -->

## Implementation Notes

<!-- SECTION:NOTES:BEGIN -->
## Round 1
A card parks in Question via `task ask` (main.go:409 → setPhase), and the daemon tick already sees every Question card (internal/daemon/daemon.go:190). Checked: `herdr notification show` takes title, --body, --position, --sound only — no click action, so a popup cannot open the pane; that unknown is settled.
Files: internal/daemon/daemon.go, internal/herdr/herdr.go:277, internal/grill/grill.go. Linked: ADR 0015, ADR 0010.

1. Where does "already announced" live, so a restart (the plugin re-execs on upgrade) does not re-pop every Question card?
   A. a state-dir marker keyed by card id + round number (reco)
   B. daemon memory only — a restart re-pops every card in Question
   T. try and show me

2. What does the popup read?
   A. title `answer owed — docket/TASK-93`, body `round 2 — A card in Question raises a Herdr popup` (reco)
   B. title `question — docket/TASK-93`, body `round 2 · A card in Question raises a Herdr popup`
   T. try and show me

3. Which sound?
   A. request, the same one a merge the owner owes uses (reco)
   B. done
   C. none
   T. try and show me

4. A second round lands on a card already announced: pop again?
   A. yes, each new round pops once (reco)
   B. no, one popup per card ever
   T. try and show me

5. A fleet paused with `herdr-docket pause`: still pop, since the queue still waits on you?
   A. yes, the waiting does not stop when scheduling does (reco)
   B. no, a paused daemon stays silent
   T. try and show me
<!-- SECTION:NOTES:END -->
