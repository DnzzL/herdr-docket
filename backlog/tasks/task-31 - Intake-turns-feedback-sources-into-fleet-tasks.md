---
id: TASK-31
title: Intake turns feedback sources into fleet tasks
status: To Do
assignee: []
created_date: '2026-09-21 19:12'
updated_date: '2026-10-01 09:38'
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
<!-- SECTION:NOTES:END -->
