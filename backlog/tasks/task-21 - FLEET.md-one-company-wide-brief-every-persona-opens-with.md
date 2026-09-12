---
id: TASK-21
title: 'FLEET.md: one company-wide brief every persona opens with'
status: To Do
assignee: []
created_date: '2026-09-12 15:09'
labels: []
dependencies: []
priority: low
ordinal: 21000
---

## Description

<!-- SECTION:DESCRIPTION:BEGIN -->
Paperclip's goals cascade from company to agent. The fleet has personas per agent and nothing above them: shared context (what the product is, who the human is, what never to do) is repeated in every AGENT.md or missing from some. One optional file, ~/fleet/FLEET.md, prepended to every persona by prompt.Assemble when present. Absent, the prompt is byte-for-byte what it is today. No cascade, no goals tree: a brief a human writes once and every agent reads.

herdr-fleet init writes a commented example so the file's existence is discoverable.
<!-- SECTION:DESCRIPTION:END -->

## Acceptance Criteria
<!-- AC:BEGIN -->
- [ ] #1 When ~/fleet/FLEET.md exists its body precedes the persona in the assembled prompt; when absent the prompt is unchanged, both pinned by the prompt test
- [ ] #2 herdr-fleet init scaffolds a commented FLEET.md example
- [ ] #3 README's The model section names the file and what belongs in it
- [ ] #4 go test ./... is green
<!-- AC:END -->
