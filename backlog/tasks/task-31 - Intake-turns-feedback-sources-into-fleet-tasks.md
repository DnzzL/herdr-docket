---
id: TASK-31
title: >-
  Intake turns feedback sources into fleet tasks
status: To Do
assignee: []
created_date: '2026-09-21 19:12'
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
