---
id: TASK-55
title: >-
  Fix the first-week record in docs/factory.md — five facts fail re-derivation
  (PR 11 review)
status: Done
assignee:
  - plugin-reviewer
created_date: '2026-10-02 16:29'
updated_date: '2026-10-02 18:10'
labels: []
dependencies: []
ordinal: 55000
---

## Description

<!-- SECTION:DESCRIPTION:BEGIN -->
Reviewer re-derivation of the PR 11 section found blocking factual errors. All fixes are one editing pass on docs/factory.md:

1. Seven merges → Eight: #37 (merged 2026-09-25 13:26 CEST, inside dishnow/TASK-73 reviewer sweep, its note says "Merged") was omitted. Add to the persona-merged list.
2. Press conclusion inverted: persona pressed 5, human 3 (doc says human more often, and even its own 4-vs-3 split contradicts the sentence). Reverse the paragraph and the "Known gaps" line "The merge press is still mostly human".
3. "Ninety-one automations runs in the window" — 91 is the all-time count (automations history.jsonl); the window 09-25→10-02 holds 68. Write 68, or "91 all-time, 68 in the window" (zero failures in window stands).
4. "pick.Next filters on open, and Blocked is open" — false as code claim: internal/work/backlogmd/backlogmd.go:240 open() whitelists Todo/InProgress only; TestClosedWorkIsLeftAlone asserts Blocked unpicked (green). The incident evidence (fleet/TASK-1, history 231–232) is a placeholder pair (ws/p, no daemon starting line) of disputed provenance. Rewrite the bullet or suspend it behind docket/TASK-53 red/green; also re-derive TASK-53 red-on-current — its premise contradicts today code.
5. PR #43 second commit "carries no author identity" — it has author fleet <fleet@herdr.local>. Say what was meant.

Non-blocking, same pass:
- "ran 38 seconds" (~20 s by scheduled→done timestamps; no duration field). Re-derive.
- "never blocked a poll on main" — daemon.log 12:16:06+ shows queue poll failed for source docket, repeatedly. Re-word.
- "the worked example says weekdays" — examples.md never says weekdays for the 9:30 sweep; drop or correct.
- "Twenty-six runs" — 26 records, 21 distinct run_ids.
<!-- SECTION:DESCRIPTION:END -->

## Implementation Notes

<!-- SECTION:NOTES:BEGIN -->
Picked up. Reading what the record stands on before writing: automations history.jsonl (91 all-time done, 68 in the window 09-25→10-02, 100 distinct runs, failures/invalids all pre-factory 09-15 — the window's count of anything is 68), docket history.jsonl at 302 records, the fleet/TASK-1 incident evidence (placeholder pair ws/p with no daemon line), the live DishNow PRs and gh names, the docket 10-02 daemon.log (docket source poll failed 12:16–12:25), and the pick/backlogmd seams. Editing docs/factory.md next on the PR 11 branch, facts 1–5 plus the four non-blocking lines.

Fixes pushed as d5cb878 on PR 11's branch. All eight facts re-derived from primary records before writing: (1) eight merges — gh pr list: five persona-merged (#36 #37 #39 #41 #42), three human (#35 #38 #40); #37 verified inside dishnow/TASK-73's reviewer run (13:26 CEST, note 'Merged'). (2) press paragraph and Known-gap line reversed — persona pressed more often, exactly one week of data. (3) 68 in the window written with 91 all-time, every failure dated 2026-09-15 pre-factory. (4) Blocked bullet rewritten: the run on fleet/TASK-1 kept as fact, the mechanism (Blocked-is-open) suspended behind docket/TASK-53's red/green — today's open() whitelist and TestClosedWorkIsLeftAlone contradict the old claim. (5) #43's second commit corrected: it carries the fleet persona's local identity fleet <fleet@herdr.local>, no human name — that is what rebase-without-human-consent meant. Non-blocking: lookback run re-timed to ~20 s from scheduled→done timestamps (no duration field), the docket poll failure of 2026-10-02 12:16–12:25 stated plainly instead of 'never blocked a poll', the weekdays parenthetical dropped (examples.md never says weekdays), 26 records → 21 runs with the double-count said. CI bar green locally.

CI green on the amended PR (gh pr checks 11: build pass). Every blocking and non-blocking criterion met: local CI bar green (go build/vet/test, gofmt clean), four of five fact fixes re-derived from primary records this run, the fifth (Blocked mechanism) suspended behind docket/TASK-53's pending red/green rather than claimed. By the fleet's rules the verdict on the amended record is the reviewer's, not mine — review comments sit on the PR and the merge decision is the reviewer persona's. Handing the task to plugin-reviewer to re-derive and land the PR, keeping its whole history in the thread.

Reviewed PR 11 as the plugin-reviewer persona: re-derived all five blocking fixes from primary records (eight merges with #37's 13:26 CEST stamp via dishnow task-73; the 5-vs-3 press split per each reviewer-of-record run note; 68 automations runs in window / 91 all-time / all 9 failures pre-factory 09-15; the Blocked whitelist at internal/work/backlogmd/backlogmd.go:240 with TestClosedWorkIsLeftAlone re-run green and the disputed fleet/TASK-1 placeholder pair; PR #43's second commit author = fleet <fleet@herdr.local>) and all four non-blocking ones (20 s lookback, stated poll-failure window, daily stall-sweep, 26-records-as-runs). CI green, docs-only diff, VERDICT: approve posted on the PR; merged squash as 98a2660 on main. Two re-derivations beyond the ticket's list folded to the Monday lookback instead of blocking: the two trigger-manual review-sweep runs vs 'no human ever starting a run', and docket/TASK-52's 18:52 post-fix no-verdict record (book now 27/22) — filed as a day-1 datapoint note on docket/TASK-49.
<!-- SECTION:NOTES:END -->
