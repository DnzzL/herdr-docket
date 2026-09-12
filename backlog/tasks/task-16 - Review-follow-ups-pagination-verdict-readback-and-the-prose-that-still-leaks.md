---
id: TASK-16
title: >-
  Review follow-ups: pagination, verdict readback, and the prose that still
  leaks
status: To Do
assignee: []
created_date: '2026-09-12 10:17'
labels: []
dependencies: []
priority: medium
type: task
ordinal: 16000
---

## Description

<!-- SECTION:DESCRIPTION:BEGIN -->
Findings from the seam/adapters/vocabulary viability review, gathered into one ticket.

The port itself carries one load-bearing gap: List() has no cursor, so the Basecamp adapter silently truncates at the API's default page size (~50). It is invisible at personal-fleet scale and interface-level in cost, which is exactly why it needs a decision recorded rather than rediscovery.

The read path carries the other gap: the write side puts a verdict into Basecamp (Close comments <Verdict: failed>), but the read side never reads it back, so a task the fleet closed as failed shows Phase Done under the board's Done heading. Reading the comment back is one request per task; whether to spend it is the decision.

And the prose: PhaseRunning's value is In Progress, which is Backlog.md's word for the phase, not the fleet's; README.md and skills/fleet-tasks/SKILL.md still say backlog and status about twenty times, because the prose ticket's ACs stopped at internal/.
<!-- SECTION:DESCRIPTION:END -->

## Acceptance Criteria
<!-- AC:BEGIN -->
- [ ] #1 Basecamp List paginates its todolists, or the truncation limit is recorded in the ADR as a deliberate limit
- [ ] #2 A Basecamp task closed as failed shows Failed, or the verdict-readback request is left unspent with the decision recorded
- [ ] #3 PhaseRunning's name and its In Progress value agree, in one direction or the other
- [ ] #4 README and skills/fleet-tasks speak the core's vocabulary where they describe the fleet, Backlog.md's product names kept where they are accurate
- [ ] #5 go test ./... is green
<!-- AC:END -->
