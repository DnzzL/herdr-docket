---
id: TASK-97
title: A create policy for the cards the factory files itself
status: Triage
assignee: []
created_date: '2026-10-28 15:45'
labels: []
dependencies: []
ordinal: 106000
---

## Description

<!-- SECTION:DESCRIPTION:BEGIN -->
The fleet has a merge policy (merge: auto, CODEOWNERS, critical) but no create policy. Agents already file cards on their own (the verifier filed TASK-89). Unchecked, the factory grows its own backlog and every agent-filed card turns into a grill round for the owner. To settle: who may create (any agent, the verifier only, none); where an agent-filed card lands (Triage, or a column the daemon never picks until the owner says yes); a cap on open agent-filed cards; how a card says who filed it.
<!-- SECTION:DESCRIPTION:END -->
