---
id: TASK-7
title: Prompt protocol speaks the fleet CLI; criteria are read-only
status: To Do
assignee: []
created_date: '2026-09-10 20:10'
labels: []
dependencies:
  - TASK-6
ordinal: 7000
---

## Description

<!-- SECTION:DESCRIPTION:BEGIN -->
Change what the agent is told. Reporting becomes `herdr-fleet task done|fail|block <id> --note "..."` and `herdr-fleet task note`; acceptance criteria are read-only (the adapter fills them for the prompt, the agent's met/not-met verdict and evidence go in the closing comment); BACKLOG_CWD disappears (the CLI resolves the fleet dir; a --fleet override is honoured); --check-ac retires. Update the fleet-tasks skill to match.
<!-- SECTION:DESCRIPTION:END -->

## Acceptance Criteria
<!-- AC:BEGIN -->
- [ ] #1 Assemble emits the new verbs and no longer mentions BACKLOG_CWD or --check-ac
- [ ] #2 acceptance criteria are rendered for reading only; the prompt never asks the agent to tick them
- [ ] #3 the prompt tells the reviewer not to trust checkboxes and to put the verdict and evidence in the closing comment
- [ ] #4 skills/fleet-tasks matches the new protocol
- [ ] #5 go test ./... green
<!-- AC:END -->
