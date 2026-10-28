---
id: TASK-86
title: The grill is a card state
status: In Progress
assignee: []
created_date: '2026-10-28 13:29'
labels: []
dependencies: []
ordinal: 86000
---

## Description

<!-- SECTION:DESCRIPTION:BEGIN -->
## Problem

An unclear idea needs a grill, and today a grill is a live chat session at a laptop. The owner steers from a phone in two-minute gaps, so the grill never happens, or an agent guesses the product decisions.

## Solution

The grill is a card state, not a session. A card in the `triage` status is routed to the queue's `triager` agent. The triage run either (a) finds the idea clear (it can write Done when + Verify by without settling a product decision, and every assumption goes under `ASSUMPTIONS:`), rewrites the card body as the spec, publishes its tickets per `docs/agents/issue-tracker.md`, and hands the card to the author in `To Do`; or (b) posts the next round of questions as a note and moves the card to `question`. The owner answers with `herdr-docket task answer <id> "1.A 2.T 3.free text"`, which posts the answer and moves the card back to `triage`. The last round is always "Build on these assumptions? A yes / B change …"; only a yes publishes. A `grill` label forces (b).

Round format (a note on the card, readable without a tool):

```md
## Round 2
1. What happens to the audio when it is sent?
   A. attached as a file (reco)
   B. text in a comment
   T. try and show me
```

Answer format (a note): `## Answer 2` then `1.A 2.T 3. keep it 30 days`.

## Done when

An idea filed in `triage` with an unclear question comes back in `question` with a Round; `task answer` sends it back; a confirmed round turns it into a `To Do` card with tickets, assigned to the author.

## Verify by

- `go test ./...` green, including: a triage card routes to the triager, never to its assignee; a `question` card is never picked; `task answer` posts `## Answer N` for the latest round and moves the card to triage; `task answer` on a card with no round fails.
- The shipped `triager` role names the clear/uncertain rule, the Round format, the final confirm round and the `grill` label.

## Out of scope

The `T` option running a prototype (v1: free text the triager interprets). The TUI (spec 4).
<!-- SECTION:DESCRIPTION:END -->
