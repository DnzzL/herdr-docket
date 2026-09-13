---
id: TASK-22
title: >-
  task create cannot write acceptance criteria, so agents reach past the fleet
  CLI
status: To Do
assignee: []
created_date: '2026-09-13 16:51'
labels: []
dependencies: []
ordinal: 22000
---

## Description

<!-- SECTION:DESCRIPTION:BEGIN -->
`herdr-fleet task create` takes a title, `-d`, `-a` and `-s`. It cannot write an acceptance criterion, and the port has no verb for one either.

That gap is not theoretical: two personas already route around it. `dev` and `reviewer` both hand work on by creating a task with criteria, and both had to be written to call `backlog task create --ac` from the repo root instead of the fleet CLI. It works today only because a source can now be the project's own board — the moment a fleet works a queue that is not the agent's checkout, the workaround stops working and the criteria are simply lost.

Why it matters more than convenience: the fleet's own prompt tells every agent that a follow-up task is how work is handed on, and several personas say a follow-up with no acceptance criterion gives the next run no bar to meet, which is how a sweep silently dies. The CLI cannot express the thing the prompt insists on.

The design question this opens, and the reason it is not a one-liner: `work.Task` already carries `Criteria []Criterion` on the way out, read-only. Writing them means either widening `Create` on the port — which every adapter must then answer for, and a Basecamp to-do has no such field — or a fourth optional capability beside Phaser and Assigner. Decide that before writing the flag.

Not urgent. Raised while writing the personas for v0.3.0; parked deliberately until the first real audit run comes back, because what that run files will show whether criteria written by an agent are worth the port change.
<!-- SECTION:DESCRIPTION:END -->

## Acceptance Criteria
<!-- AC:BEGIN -->
- [ ] #1 herdr-fleet task create can attach acceptance criteria at creation, or the fleet states plainly why it never will
- [ ] #2 No shipped persona reaches past the fleet CLI to write a task any more
- [ ] #3 An adapter that cannot store criteria fails or degrades in a stated way, never silently drops them
<!-- AC:END -->
