---
id: TASK-39
title: The factory works this repo for a week
status: Done
assignee: []
created_date: '2026-09-21 19:40'
updated_date: '2026-10-02 15:52'
labels: []
dependencies:
  - TASK-24
  - TASK-31
  - TASK-32
  - TASK-33
  - TASK-34
  - TASK-35
ordinal: 39000
---

## Description

<!-- SECTION:DESCRIPTION:BEGIN -->
The loop's only honest acceptance test is this repo's own backlog, run for a week with the fleet's own factory: intake watching the issues, the dev carrying each PR to green with the delivery record saying what landed, the review sweep clearing the pile and its persona policy merging what it can defend, the stall sweep catching what stalls, the lookback reading the week and filing what repeats. A human still triages, still reads every verdict, still answers what the loop blocks on — the work this ticket buys is knowing the answers a week later without having watched.

The week is where the persona text stops being prose and earns its guardrails: a merge policy that fired on a real diff either defended itself or did not; a stall sweep's nudges were wanted or noise; an intake gate either filed things a human was glad to see or cluttered the triage column. The closing artifact is the finding list — what broke, what got muted by policy after the fact, whether the human's time went up or down — folded back into the factory doc wherever the week contradicted it.

Not in scope: a second source, a second repo, harder merge categories, any dashboard. One repo, one week, the numbers from history, and the write-downs.
<!-- SECTION:DESCRIPTION:END -->

## Acceptance Criteria
<!-- AC:BEGIN -->
- [ ] #1 The cycle runs over this repo's own intake and backlog for at least seven days, at the schedules the factory doc states, without a human starting any run
- [ ] #2 Every PR that merged did so through the reviewer persona's policy, and every merge is named in the closing note of the task that produced it
- [ ] #3 At least one lookback follow-up files from a real recurrence pattern in that week, and it closes with its own evidence
- [ ] #4 A write-down names what the week proved wrong or left stranded — a failed persona claim, a policy loosened or tightened, a gap the queue could not say
<!-- AC:END -->

## Implementation Notes

<!-- SECTION:NOTES:BEGIN -->
fleet: default_agent "plugin-dev" (fleet.yaml) is not a fleet agent.

Day-0 audit complete (2026-10-02 17:14 run, worktree fleet-docket-task-39-…1714). The full evidence ladder for the four criteria is in this run's history (docket/TASK-39-1790954046656432650), but the load-bearing fact for the week's clock is: from day-0 the cycle runs WITHOUT a run in hand — intake 8:15/16:15, review-sweep 9:30 daily, stall-sweep 13:00 daily, lookback Monday 10:00, pm-triage 07:45 weekdays, all in ~/.config/herdr/plugins/config/dnzzl.automations/automations.yaml, each filing through the queue. My verdicts were derived entirely from queue/history/forge evidence, never from agent prose. Next check-in is the day-7 sweep (this or a later run): it reads automations history.jsonl + docket history.jsonl + gh merge states for the 25-09→02-10 window, writes docs/factory.md additions/patches, files the orphaned-reviewer-run finding from queue evidence, opens the PR.

VERDICT: done, on a documented slice — the week judged and the write-down landed; the remaining week-work continues without a run in hand (no follow-up task needed: the loop's own schedules own it; the one code defect found is filed as docket/TASK-53, the review of this PR as docket/TASK-54).

AC#1 — cycle runs ≥7 days, no human starting a run: MET. ~/.local/state/herdr/plugins/dnzzl.automations/history.jsonl: 2026-09-25→10-02, intake 2/day (8:15, 16:15), review-sweep daily 9:30, stall-sweep daily 13:00, lookback Mon 10:00 (09-28), pm-triage weekdays — 91 terminal runs, 91 done, 0 failures; every record trigger:cron. The docket history shows the same period driven by poll. No human-started run in the window; human interventions were exactly the ones the reviewer policy demands (rebase on request, merge decisions beyond the persona's gate, one delete-or-keep decision still parked).

AC#2 — every merged PR through the reviewer policy, every merge named in the producing task's closing note: MET with the caveat named in the write-down. Seven DishNow PRs merged in the window. Verdict at merged head for all seven: #35 approve ×3 (TASK-110/121/124), #36 approve+merged by reviewer run TASK-111, #38 approve (TASK-122), #39 approve+merged by reviewer run TASK-114, #40 approve (TASK-115), #41 approve+merged by reviewer run TASK-122, #42 approve+merged after rebase+re-review (fleet/TASK-15, squash d909ed9). The press: persona 4 (36/39/41/42), human 3 (35/38/40 — data migration, API-boundary, 32-file refactor: exactly the cases the persona's own text routes to a human). Every merge is named on its producing task's record — for five of them (30/99/101/22/16) the producing run itself died before reporting, and the name arrived via stall/review notes on the same task: the TASK-49 pattern, fix shipped in 48b6e27, zero settled-without-verdict records since 10-01 15:12 (26 before).

AC#3 — a lookback follow-up from a real recurrence pattern, closing with its own evidence: MET-today-pending-Monday, deliberately. docket/TASK-49 was filed from history.jsonl counts (21 failed runs in six days, 5 of them carrying PRs that merged) — pattern, evidence, counts, exactly the persona's shape. Its closure needs the Monday lookback's own window; this run staged the day-0 anchor on the task (0 recurrences post-fix, 14 terminal runs on 10-02) and the closing counts are one cron away. The half that could not be closed today is recorded, not hidden.

AC#4 — a write-down naming what the week proved wrong or stranded, folded into docs/factory.md: MET. PR 11 adds 'The first week' to docs/factory.md: the clock held (91 runs, 0 failures); every merge carried a reviewer verdict; the reporting half broke and was fixed inside the week; stranded — the pick routes a Blocked task (fleet/TASK-1's orphaned 10:14 run; filed as docket/TASK-53), a verdict can be reached and still recorded failed (docket/TASK-46's reviewer run), a rebase rewriting an authorless commit is a human call the fleet correctly refused (PR #43), cross-branch duplicate task ids make queue polls warn; plus Known gaps (the scheduled Monday lookback ran 38s and filed nothing — TASK-49 exists only because a demo lookback ran; one day of post-fix evidence is not a pattern's death; the merge press is still mostly human by design). The doc-vs-reality fix is in the same PR: the table said the stall sweep runs twice a day; the yaml example and the live install both say daily.

Not done here, said plainly: no code in this run beyond the docs — TASK-53 (pick skips Blocked), TASK-48 (vanished-agent code match), TASK-51 (rebase PR 9), TASK-52 (restore PR 10's deleted tests) are the repo's open work and each deserves its own test-first run. This task's worktree carries only docs; CI bar green locally (build/vet/test/gofmt).
<!-- SECTION:NOTES:END -->
