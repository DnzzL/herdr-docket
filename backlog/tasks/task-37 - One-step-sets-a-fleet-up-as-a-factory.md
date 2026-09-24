---
id: TASK-37
title: >-
  One step sets a fleet up as a factory
status: To Do
assignee: []
created_date: '2026-09-21 19:12'
labels: []
dependencies:
  - TASK-31
  - TASK-33
  - TASK-36
ordinal: 37000
---

## Description

<!-- SECTION:DESCRIPTION:BEGIN -->
The distribution wedge is the last item: the video's factory spreads because its skills repo installs with one CLI. Here the equivalent is a single command that turns a fleet that runs tasks into a fleet that runs the loop — writing the factory personas from the bundled shapes, appending the automations.yaml entries that drive them, and pointing the user at docs/factory.md for what they are agreeing to run. The pieces all exist in skills or docs by the time this lands; the work is one install path over them, not a second canon.

Scope guards, because this is where the repo would be tempted to grow a platform: the init writes personas and schedules, exactly once per name, refusing to overwrite a persona a user already has — a fleet that already runs a reviewer is not re-fenced mid-calendar. Nothing is polled, nothing connects anywhere, nothing is required: a fleet without the automations plugin gets its personas and a note that they await schedules, and a human still runs the first intake dry-run before the schedule is trusted. Scheduling stays the automations plugin's job and the fleet records no copy of it.
<!-- SECTION:DESCRIPTION:END -->

## Acceptance Criteria
<!-- AC:BEGIN -->
- [ ] #1 One subcommand writes the factory personas into an existing fleet dir, each named once and never overwriting one that exists, and prints what it wrote
- [ ] #2 Where the automations plugin is present its config gains the driving entries, appended rather than rewritten; where it is absent the user is told the personas await schedules and nothing else is written
- [ ] #3 The fleet sources block and statuses are untouched: init configures the loop only over queued work the user already admits the fleet to
- [ ] #4 docs/factory.md shows the one command as the setup path, and the init's output points the user at the dry-run-first intake rule before its schedule is switched on
<!-- AC:END -->
