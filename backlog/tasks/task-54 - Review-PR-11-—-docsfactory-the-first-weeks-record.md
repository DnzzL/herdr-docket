---
id: TASK-54
title: 'Review PR 11 — docs(factory): the first week''s record'
status: Done
assignee:
  - plugin-reviewer
created_date: '2026-10-02 15:52'
updated_date: '2026-10-03 20:12'
labels: []
dependencies: []
ordinal: 54000
---

## Description

<!-- SECTION:DESCRIPTION:BEGIN -->
Docs-only PR: https://github.com/DnzzL/herdr-docket/pull/11 (produced by docket/TASK-39, docket/TASK-53 filed as its one code follow-up). The diff adds "The first week — 2026-09-25 to 2026-10-02" to docs/factory.md and fixes one wrong table cell (stall sweep daily, not twice a day). Every factual claim in the section is meant to be re-derived, not trusted: run counts from ~/.local/state/herdr/plugins/dnzzl.automations/history.jsonl (91 terminal done, 0 failures 09-25→10-02; 6 failures are 09-15 pre-factory), merge verdicts from gh pr view 35/36/38/39/40/41/42 -R DnzzL/DishNow-v2 and the named dishnow tasks (30/99/101/22/16/31/65), the settled-without-verdict count (26, last 2026-10-01 13:37; fix 48b6e27 at 15:12; zero since) from the herdr-docket history, the Blocked-pick incident from the fleet/TASK-1 notes and internal/pick/pick.go. Re-run the CI bar locally; then merge per policy — the diff is doc-sized, which your gate allows.
<!-- SECTION:DESCRIPTION:END -->

## Implementation Notes

<!-- SECTION:NOTES:BEGIN -->
Review of PR 11 complete: all claims re-derived (history.jsonl×3, daemon.log, live automations yaml, internal/pick+backlogmd code, PR #43 commits, examples.md). CI bar green locally (go build/vet/test, gofmt clean), gh pr checks build pass, mergeable CLEAN. Five blocking factual findings (eight merges not seven — #37 persona-merged omitted; press conclusion inverted — persona 5 vs human 3; 91 is all-time, 68 in window; 'Blocked is open' false against open() whitelist + green pick_test; PR #43 commit does carry author identity fleet@herdr.local) + four non-blocking (38s≈20s, docket poll did fail 12:16+, examples.md never says weekdays, 26 records/21 runs). Review posted on the PR with VERDICT: block. Fixes filed as docket/TASK-55 (assigned plugin-dev). Not merging: blocking findings stand; blocked on the follow-up.

Blocked, not merged: the docs PR fails its own evidence bar. Five blocking re-derivation failures posted on https://github.com/DnzzL/herdr-docket/pull/11 (VERDICT: block), concrete fixes filed as docket/TASK-55 assigned plugin-dev. Blocking is the author-fix loop: once TASK-55 lands as a follow-up commit on the PR branch and the amended numbers are re-derived, re-review and merge. CI itself is green (docs-only diff; gh pr checks build pass, local go build/vet/test + gofmt clean).

fleet: the run's workspace w1 (pane w1:pA) is left open — jump in to resume.

Stall sweep (fleet/TASK-21), pass 7, stall type 2 — the block you closed on was answered by an event no note here records: you were 'blocked on the follow-up' docket/TASK-55. TASK-55 closed Done 2026-10-02 20:10, and its final note is the plugin-reviewer's own re-derivation of all five blocking fixes — VERDICT: approve posted on PR 11, squash-merged to main as 98a2660 (gh, merged 2026-10-02T18:10:32Z for PR 11 base... correction: PR 11 merged at 2026-10-02T18:10:32Z per gh pr list; the approve+merge is recorded in TASK-55's closing note). Either way your author-fix loop finished without you; the row still reads blocked on TASK-55. Who it waits on: the human — route or close this parked row. Nudge only: I do not reopen or close.

Stall sweep (fleet/TASK-21), pass 7, wording correction to my previous note — the facts, cleanly: PR 11 merged 2026-10-02T18:10:32Z (= 20:10:32 local, gh mergedAt, merge commit 98a2660, head d5cb878 = docket/TASK-55's fix commit), and docket/TASK-55 closed Done at 20:10:48 local with the plugin-reviewer's approve on PR 11 in its closing note. My previous note garbled that timestamp mid-sentence; content unchanged — your block on TASK-55 is answered, no note here recorded it.

Cleanup 2026-10-03: obsolete — PR #11 is MERGED.
<!-- SECTION:NOTES:END -->
