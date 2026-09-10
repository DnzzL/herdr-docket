---
id: TASK-5
title: 'Adapter conformance suite: one contract every Source must pass'
status: Done
assignee: []
created_date: '2026-09-10 20:09'
updated_date: '2026-09-10 20:16'
labels: []
dependencies:
  - TASK-4
ordinal: 5000
---

## Description

<!-- SECTION:DESCRIPTION:BEGIN -->
Add internal/work/worktest: a reusable test package that runs the Source contract against any adapter (List/Get/Create/Comment/Close, verdict preserved on the item, phase written best-effort and never read back). backlogmd must pass it; the Basecamp adapter will be held to the same suite. This is what makes a second adapter safe to add.
<!-- SECTION:DESCRIPTION:END -->

## Acceptance Criteria
<!-- AC:BEGIN -->
- [ ] #1 worktest.Contract(t, factory) exercises List, Get, Create, Comment and Close with the real verdict vocabulary
- [ ] #2 the suite fails loudly for a Source that drops the verdict, leaves an item open after Close, or makes Get disagree with List
- [ ] #3 backlogmd passes the suite
- [ ] #4 go test ./... green
<!-- AC:END -->
