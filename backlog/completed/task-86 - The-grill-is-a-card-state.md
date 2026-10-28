---
id: TASK-86
title: The grill is a card state
status: Done
assignee: []
created_date: '2026-10-28 13:29'
updated_date: '2026-10-28 14:07'
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

## Implementation Notes

<!-- SECTION:NOTES:BEGIN -->
plugin-dev run: PLAN around the seam, then built. Seams: (1) work.Vocabulary gains triage/question (both-or-neither, both Open) and work.Task gains Triaging/Asking, filled by backlogmd+notara; (2) two optional port capabilities — Griller.Move(Triage|Question|To Do) and BodyWriter — because these words decide what runs, unlike Phaser's display-only phase (ADR 0012's contract kept); backlogmd/notara implement, multi forwards, CLI refuses a queue without them; (3) SourceConfig.Triager + Defaults.TriagerFor: pick routes a triaged card to the triager whatever assignee it names, never picks Asking, and reports a triaged card whose queue names no triager (daemon note); (4) prompt gets the grill protocol (clear/uncertain rule, Round format, final confirm round, grill label) and reconcile treats a parked question card as a clean open-on-purpose ending while a card left in Triage ends Failed (no next-tick spin); claim leaves a triaged card's status alone; (5) verbs task ask/answer/move/body, shipped roles/triager.md (drift-pinned to docs/examples.md §5), ADR 0023, CONTEXT.md, commands.md, changelog.d/TASK-86.md. Verified myself: full bar green (go build/vet/test ./..., gofmt -l . empty); card Verify-by tests all present and green (routing, question-never-picked, answer posts ## Answer N for the latest round then Tri, answer-with-no-round fails, triager role states the four rules); live smoke on a real Backlog.md queue in /tmp (binary to /tmp, temp fleet.yaml): create in Triage -> task ask wrote ## Round 1 + Question -> task answer "1.A" wrote ## Answer 1 + Triage -> task body/assign/move left it open in To Do with the spec body. Not done in this slice: no live daemon-tick demo (unit-tested at pick/reconcile); T option detection left as free text per Out of scope.

Delivered PR #60. Verdict per criterion: Done-when — an idea filed in Triage with an unclear question comes back in Question with a Round (live smoke: task ask wrote '## Round 1' and moved the real Backlog.md card to Question); task answer sends it back (wrote '## Answer 1', card in Triage); a confirmed round turns it into a To Do card with tickets, assigned to the author (move todo + assign + body write demonstrated live; tickets are published to the repo's tracker per docs/agents/issue-tracker.md by the shipped triager role). Verify-by — go test ./... green including the four named tests (triage routes to triager never assignee; question never picked; answer posts ## Answer N for the latest round and moves to triage; answer on a card with no round fails), and the shipped triager role names the clear/uncertain rule, the Round format, the final confirm round and the grill label (TestTheShippedTriagerNamesTheGrillRules). CI: build pass on head; full local bar green, gofmt clean. Structural decision recorded in ADR 0023; user-visible change has changelog.d/TASK-86.md.

verdict FAIL on https://github.com/DnzzL/herdr-docket/pull/60: FAIL. Bar green (go build/vet/test ./..., gofmt -l . empty) and the five named tests pass, but two acceptance gaps. 1) Clear path closes the card: internal/prompt/prompt.go:167-171 tells the triager to end with 'herdr-docket task done ... only if the idea is clear and specced', and main.go:730-737 closes every done without a PR. The card just moved to To Do and assigned is then Done, so the author never picks it; this contradicts the shipped role and Done-when. Fix: remove task done from the grill ending (and the verdict-protocol fallback), make the endings the move to todo / ask, and add a runner test that a triaged card body-written, assigned and moved to To Do stays open. 2) triager: on a singular source: is ignored: internal/fleet/defaults.go:24-48 only reads s.Sources, while a source: queue has unprefixed ids (SourceOf empty), so TriagerFor always returns empty and the daemon calls the card untriaged even when triager: is set. Fix: seed triagerFor from s.Source under the empty key (or route through Settings.sourceOf) and add a defaults/pick test for the singular-source fleet. Minor: docs/queues.md still omits triager and the two grill statuses from its source-key table.

Fixed both FAIL findings on PR #60 (commit 3534ee3, pushed). 1) internal/prompt/prompt.go: the grill no longer offers  (nor fail/block) as an ending; its two endings are  and . New runner test TestATriagedCardSpeccedAndMovedToTodoStaysOpen covers a triaged card body-written, assigned and moved to To Do — it stays open with no verdict. Prompt test now forbids  in the grill. 2) internal/fleet/defaults.go: when there is no sources:, Defaults seeds bySource and triagerFor from s.Source under the empty key SourceOf reads, so a singular source:'s triager and default_agent now route. New TestASingularSourceRoutesWithItsOwnSettings. Minor: docs/queues.md gains the triager key and the triage/question statuses. Full bar green locally: go build ./..., go vet ./..., go test ./... all pass; gofmt -l . empty. Verify-by criteria remain met: triage routes to triager not assignee, question never picked, task answer posts ## Answer N and moves to triage, answer with no round fails, triager role names the clear/uncertain rule, Round format, final confirm and grill label.

Correction to the previous note (shell ate the backticked phrases): the grill prompt no longer offers task done / fail / block as an ending; its only endings are task move <id> todo and task ask <id>. The prompt test forbids those three closing commands with the card id. Everything else in that note stands: both verifier findings fixed, singular source triager/default now route under the empty key, docs/queues.md updated, full bar green (build/vet/test, gofmt empty).

Verifier findings fixed and pushed (3534ee3). (1) The grill prompt no longer offers task done/fail/block; a clear idea ends with task move <id> todo and an uncertain one with task ask <id>. New runner test proves a triaged card that is body-written, assigned to its author and moved to To Do stays open with no verdict; the prompt test forbids the three closing commands with the card id. (2) Defaults now reads a singular source: as the empty-key queue, so its triager: and default_agent: route — TestASingularSourceRoutesWithItsOwnSettings; a triaged card on that fleet is no longer called untriaged. Minor: docs/queues.md lists triager and the triage/question statuses. Local bar replayed on head: go build ./..., go vet ./..., go test ./... all green, gofmt -l . empty. Task Verify-by still met: triage routes to the triager never assignee; question never picked; task answer posts ## Answer N for the latest round and returns the card to triage; answer with no round fails; shipped triager role names clear/uncertain, Round format, final confirm and grill label.

fleet: this run's delivery could not be read — workspace wNK (pane wNK:p1) is kept, unverified. The verdict stands.

verdict PASS on https://github.com/DnzzL/herdr-docket/pull/60: PASS. Re-derived on head 3534ee3 in the provisioned worktree. Bar: go build ./..., go vet ./..., go test -count=1 ./... all green; gofmt -l . empty. Named Verify-by tests pass: TestATriagedCardRoutesToTheTriagerNotItsAssignee, TestAQuestionCardIsNeverPicked, TestAnswerPostsTheLatestRoundNumberAndReturnsToTriage (posts ## Answer 2, moves PhaseTriage), TestAnswerWithNoRoundFails (touches nothing), TestTheShippedTriagerNamesTheGrillRules (clear/uncertain, ## Round 2, Build on these assumptions?, grill). Drift test TestTheRolesInitWritesAreTheOnesTheDocsShow also passes for the triager role. Both prior FAILs are fixed and proven: internal/prompt/prompt.go grill() now ends only with task move <id> todo / task ask <id> and forbids done/fail/block (TestATriagedCardGetsTheGrillProtocolNotTheVerdictOne), TestATriagedCardSpeccedAndMovedToTodoStaysOpen asserts no verdict and an open card; internal/fleet/defaults.go seeds triagerFor/bySource from a singular source under the empty key, proven by TestASingularSourceRoutesWithItsOwnSettings. Live replay on a real Backlog.md queue (binary built from head to /tmp, temp HERDR_PLUGIN_CONFIG_DIR): fleet init wrote statuses To Do/In Progress/Triage/Question/Blocked/Failed/Done; create -> task move TASK-1 triage -> task ask wrote ## Round 1 and card in Question; task answer "1.A 2.T 3. keep it 30 days" wrote ## Answer 1 and card in Triage; task answer on a fresh card exited 1 with "no ## Round to answer"; task body + assign dev + move todo left the card open in To Do with the spec body; task list showed a triaged card routed to triager despite Assignee dev. Non-blocking follow-up filed as docket/TASK-89: the triager cannot actually see work.Task.Labels (prompt head and task view omit them), so the grill label rule is stated but not observable; TASK-86 Verify-by only requires the role to name it, which it does.

fleet: https://github.com/DnzzL/herdr-docket/pull/60 is held for a human — internal/daemon/daemon.go is a CODEOWNERS path (/internal/daemon/).
<!-- SECTION:NOTES:END -->
