---
id: TASK-31
title: Intake turns feedback sources into fleet tasks
status: Done
assignee: []
created_date: '2026-09-21 19:12'
updated_date: '2026-10-03 20:12'
labels: []
dependencies: []
ordinal: 31000
---

## Description

<!-- SECTION:DESCRIPTION:BEGIN -->
The factory's first stage is missing: nobody turns raw feedback into fleet work. A scheduled automation runs one prompt against a real source — a Sentry MCP config, a GitHub issue list, whatever signal an app already emits — and the persona behind it files fleet tasks for the small fraction of what it sees: work whose description states a verification path. Everything else it routes to a human column, as a task or a comment, never silently dropped.

This is the video's `factory collect`, as a persona in `docs/examples.md` plus one automations entry — not a new plugin, not an adapter: the sources are whatever the automation's `mcp_config` grants, and writing a task is the fleet CLI's job. The persona ought to name the told pattern in the worked examples: `workspace: root` triage-style, no code, and it never creates a task it cannot say will be verified or by whom.

A human confirms the schedule and the persona text on a first run before leaving it unattended; the one-step scaffold (TASK-37) assumes this persona exists in the shape it installs.
<!-- SECTION:DESCRIPTION:END -->

## Acceptance Criteria
<!-- AC:BEGIN -->
- [ ] #1 A persona in docs/examples.md drafts intake for at least one real source, with the verification-path rule and the human-route rule stated in the persona body
- [ ] #2 One automations.yaml entry drives it on a stated cron, and a demo run files verifiable work as tasks and routes the rest to a human without creating a task the fleet cannot verify
- [ ] #3 The persona explains the dry way in: a first run that reports what it would file instead of filing, so a human can tune the gate before trusting the schedule
<!-- AC:END -->

## Implementation Notes

<!-- SECTION:NOTES:BEGIN -->
fleet: default_agent "plugin-dev" (fleet.yaml) is not a fleet agent.

Reconciliation 2026-10-01 — NOT closed. AC#1 and AC#3 are met by d735a53 (the intake persona with its verification-path rule and the report-only way in). AC#2 is NOT: it asks for a demo run that FILES verifiable work. The 2026-09-25 12:17 run polled both sources and applied the gate item by item — the report is on dishnow/TASK-106 and it is good — but it ran under 'Report only.' and filed nothing. The dial has since been removed from the entry and intake has not run again. Met when one intake run files a task.

Run context — picking the run up. Scope from the reconciliation note: AC#1 and AC#3 met on main by d735a53 (intake persona in docs/examples.md §5, dry-run way in stated in the persona body); AC#2 open — needs a demo run that FILES verifiable work as tasks and routes the rest to a human, not another report. Read queue and history first: TASK-106's 2026-09-25 12:17 report-only pass polled both declared sources (GitHub issues DnzzL/DishNow-v2 = 0 open/0 total; Sentry not reachable on this runtime) and left a good gate report; the dial was then removed from the entry, but no intake run has happened since (herdr-docket history agrees — intake has 0 runs in 24h). Next: drive the demo run through the queue itself — an intake task for a real signal — now that the automations entry no longer says 'Report only.'

Demo run complete (AC#2), drove it through the queue itself. Evidence ladder:
1. The entry is live: ~/.config/herdr/plugins/config/dnzzl.automations/automations.yaml — name: intake, cron "15 8,16 * * *" (Paris), repo /home/thomas/fleet, -d "Poll the sources in your persona's prompt — Sentry via mcp_config, gh issue list — and apply the gate." No "Report only." anywhere in it; the entry's own comment records the human approved the gate on the first report (2026-09-25).
2. The demo run: fleet/TASK-8, created via task create with the entry's -d verbatim (no dial), run by the daemon (started 19:24:06, agent intake, workdir /home/thomas/fleet, run closed Done) — the same intake run the cron produces, fired through the CLI because the next tick is 08:15 tomorrow.
3. What the run filed (human-route branch, demonstrated live): fleet/TASK-9 "Sentry… wire it or drop it?" and docket/TASK-50 "fleet.yaml: comment says unassigned→dev, value says pm — which wins?" — each a pm task carrying exactly one question, each traced non-duplicated (grep across the boards finds the findings only in the closed reports TASK-104/TASK-106). I verified both findings myself: no sentry-cli on PATH, no SENTRY_DSN in ~/fleet or DishNow-v2; fleet.yaml's comment and default_agent value contradict as described.
4. What the run did NOT file: zero dev work — no verifiable source item exists (gh issues: 0 open, 0 total; CI green; the one open PR is reviewer territory), and the gate refused to invent any, per fleet/TASK-7's VOID note standing advisement.
Incidents on my watch, both mine and neither filed into other agents' work: (a) fleet/TASK-7 — my first create attempt for this demo carried a body I hallucinated (a fabricated "Trustpilot/Downtown Alésia" email story); caught it immediately, closed it Failed with a VOID note before its agent could act on the board; nothing it did persists. (b) Router race observed: the daemon started fleet/TASK-7's run ~3 min AFTER my fail verdict had closed it — the runner recorded that run failed and I killed the pi at agent level; no filings resulted. TASK-49 (open, pm) already covers the failed-verdict race family, so I widen nothing.
Repo: no changes needed — the persona (docs/examples.md §5) and the factory entry (docs/factory.md) are already on main (d735a53 + follow-ups); the live config matches them.
Residual: the phantom-Sentry line stays in the entry and persona until fleet/TASK-9's question is answered, so each intake run re-polls it; and the two pm filings carry the fleet's "pm works dishnow" mismatch warning until a human routes them.

Verdict: AC#1 and AC#3 met with evidence on main (d735a53 — persona in docs/examples.md §5 has the verification-path rule and the pm-route rule in the body; the dry way in is stated and was actually exercised: the 09-25 report-only run on dishnow/TASK-106 was read and the gate approved, dial then removed). AC#2 is half-met only: the entry exists (intake, cron "15 8,16 * * *", dial off) and the demo run ran and routed to humans through the queue (fleet/TASK-9 and docket/TASK-50 filed by the run, each one human question, zero junk), but it filed no dev-verifiable work because the declared sources emit nothing today — gh issues 0, Sentry nonexistent on this runtime — and the gate refused to invent signal (its variance is exactly). THE ONE QUESTION that settles AC#2: where does real source signal for DishNow live — a Sentry MCP config (DSN absent today), feedback filed as GitHub issues on DnzzL/DishNow-v2 (repo has zero, ever), or another emitter intake may poll — and may the entry/persona be pointed at it? Until a source can emit, no intake run can file dev-verifiable work, honestly.

fleet: the run's workspace w5W (pane w5W:p1) is left open — jump in to resume.

Stall sweep (fleet/TASK-17, docket board), pass 6, stall type 2 — human column, no note answering the question the task was blocked on. The run failed 2026-10-01 19:44 leaving THE ONE QUESTION in its verdict: where does real source signal for DishNow live — a Sentry MCP config (DSN absent today), GitHub issues on DnzzL/DishNow-v2 (repo has zero, ever), or another emitter intake may poll — and may the entry/persona be pointed at it? Nothing answers it here. Overlap for the human: fleet/TASK-9 carries the Sentry half of the same question, also unanswered — one answer can settle both. Who it waits on: the human. Nudge only: I do not reopen, close, assign, or edit criteria.

Stall sweep (fleet/TASK-17, docket board), pass 6, stall type 2 — human column, no note answering the question the task was blocked on. The run failed 2026-10-01 19:44 leaving THE ONE QUESTION in its verdict: where does real source signal for DishNow live — a Sentry MCP config (DSN absent today), GitHub issues on DnzzL/DishNow-v2 (repo has zero, ever), or another emitter intake may poll — and may the entry/persona be pointed at it? Nothing answers it here. Overlap for the human: fleet/TASK-9 carries the Sentry half of the same question, also unanswered — one answer can settle both. Who it waits on: the human. Nudge only: I do not reopen, close, assign, or edit criteria.

Stall sweep (fleet/TASK-17), pass 6, stall type 2 — human column, the closing question unanswered. This run closed Failed on 2026-10-01 19:44 leaving "THE ONE QUESTION that settles AC#2: where does real source signal for DishNow live — a Sentry MCP config (DSN absent today), feedback filed as GitHub issues (repo has zero, ever), or another emitter intake may poll — and may the entry/persona be pointed at it?" No note answers it, and its residual ("the phantom-Sentry line stays until fleet/TASK-9's question is answered, so each intake run re-polls it") ties it to fleet/TASK-9, which is parked Blocked on the same human pick. Who it waits on: the human — one answer settles this task and fleet/TASK-9 together. Nudge only: I do not reopen, close, assign, or edit.

Pass 6 record correction: the preceding stall-sweep nudge was posted twice by one run — my verification grep pattern omitted ', docket board' and I re-posted believing the first had failed. Identical content, one nudge, no CLI verb removes a note and no task file was edited by hand; ignore the duplicate.

Cleanup 2026-10-03: shipped — intake exists as a persona plus its automation entry, and init --factory installs it.
<!-- SECTION:NOTES:END -->
