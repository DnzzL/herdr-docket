---
id: TASK-9
title: 'Config: a source block and one place that builds a Source'
status: Done
assignee: []
created_date: '2026-09-10 20:10'
updated_date: '2026-09-10 20:26'
labels: []
dependencies:
  - TASK-6
ordinal: 9000
---

## Description

<!-- SECTION:DESCRIPTION:BEGIN -->
fleet.yaml gains a source: block (kind: backlogmd is the default, plus per-kind settings). One function builds a work.Source from the settings, so daemon, CLI and board all resolve the queue the same way. fleet init keeps scaffolding a local backlog for the default kind and scaffolds no local backlog for a remote one.
<!-- SECTION:DESCRIPTION:END -->

## Acceptance Criteria
<!-- AC:BEGIN -->
- [ ] #1 a fleet.yaml without a source block defaults to backlogmd and keeps working unchanged
- [ ] #2 one constructor maps settings to a work.Source; no caller builds an adapter itself
- [ ] #3 herdr-docket init for backlogmd is behaviourally unchanged
- [ ] #4 go test ./... green
<!-- AC:END -->
