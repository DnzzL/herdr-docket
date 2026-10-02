---
id: TASK-40
title: >-
  A task the fleet creates lands in the queue's own pickup status, not the
  project's default
status: Done
assignee: []
created_date: '2026-09-25 09:10'
updated_date: '2026-10-01 15:45'
labels: []
dependencies: []
priority: high
ordinal: 40000
---

## Description

<!-- SECTION:DESCRIPTION:BEGIN -->
<!-- SECTION:DESCRIPTION:BEGIN -->
<!-- SECTION:DESCRIPTION:BEGIN -->
<!-- SECTION:DESCRIPTION:BEGIN -->
<!-- SECTION:DESCRIPTION:BEGIN -->
<!-- SECTION:DESCRIPTION:BEGIN -->
`herdr-docket task create` never names a status, so the backend files the task
in the project's `default_status`. The fleet, meanwhile, picks up exactly one
status per source — the mapped `todo` word. Where those two words differ, every
task the fleet creates strands in a column the daemon will never read: the
intake meta-task, the reviewer's blocking follow-ups, the lookback's pattern
fixes. The loop would stall at its first handoff, silently, and no sweep would
name it because the tasks are open, just never claimed.

The gap was found wiring a real fleet (dishnow board, own status words). The
config works around it by mapping `todo` to the project's default — honest for
that board, but it only holds while the two words happen to be the same word,
and it asks every future fleet to bend its columns to the CLI's silence.

The design question, which TASK-22 already opened from the criteria side and
which must be settled before any flag exists:

- **Widen `Create`** with a starting status — every adapter answers for it or
  refuses, and a Basecamp to-do has no column to start in.
- **A fourth optional capability** beside `Phaser` and `Assigner`: a source
  that can be started-in a status is a source that can, one that cannot says
  so, and `Create` keeps its three arguments.
- **Create, then `SetPhase(Todo)`** through the existing `Phaser` — no port
  change at all, at the cost of a second backend round trip per create (a
  GitHub board pays points) and a brief window where the task exists in the
  wrong column.
- **Fold with TASK-22** — one `Create` carrying status *and* criteria, one
  decision instead of two, if the audit that task waits for has come back.
<!-- SECTION:DESCRIPTION:END -->
<!-- SECTION:DESCRIPTION:END -->
<!-- SECTION:DESCRIPTION:END -->
<!-- SECTION:DESCRIPTION:END -->
<!-- SECTION:DESCRIPTION:END -->

## Acceptance Criteria
<!-- AC:BEGIN -->
- [ ] #1 A task created through the fleet CLI lands in the source's mapped todo status on backends that can name one (Backlog.md at minimum), so the next daemon tick can claim it
- [ ] #2 A backend with no such concept creates as it does today, and the CLI's own output says where the task landed rather than implying the fleet chose
- [ ] #3 A fleet with several sources gets the behaviour per source, each in its own words — one queue's default must not decide another queue's landing column
- [ ] #4 The shape chosen — widened Create, optional capability, SetPhase-after-create, or folded into TASK-22 — is recorded (ADR or a TASK-22 comment) with the rejected alternatives, before the first adapter changes
<!-- AC:END -->

## Implementation Notes

<!-- SECTION:NOTES:BEGIN -->
fleet: default_agent "plugin-dev" (fleet.yaml) is not a fleet agent.

Reading the queue done: backlog CLI supports --status on create; GitHub/Backlog.md adapters read freshly created work as claimable, so only Backlog.md's status whitelist strands tasks; design will be recorded in an ADR before any adapter changes.

Slice settled: optional TodoStarter capability (port keeps its 3-arg Create); backlogmd CreateTodo files new work with the backlog CLI's own --status flag in one write; MultiSource.CreateIn returns started so the CLI's output reports landing honestly (AC#2); GitHub and Basecamp untouched (they already read new work as claimable — AC#2/AC#3); ADR-0012 first.

Built and pushed: PR 6 https://github.com/DnzzL/herdr-docket/pull/6 (green build). Shape: fourth optional capability TodoStarter, Create keeps 3 args (ADR-0012, committed before the adapter change). Backlog.md files new work into the project's own pickup word in one write (--status on the backlog CLI). MultiSource.CreateIn now answers started-per-source; CLI and pane report the landing honestly.

Done. #1 met: the local build made a real create against a throwaway Backlog.md project (own words: pickup Ready, default Bench) — the file landed status: Ready, output 'created docket/TASK-1 (in To Do)'; claimability readback unit-tested against the adapter. #2 met: incapable backends (Basecamp, and any source without the capability — fakeSource) fall back to plain Create unchanged and the CLI prints 'filed in the queue's own default — the fleet did not choose the column'; Basecamp/GitHub not exercised end-to-end here, no credentials — the fallback path itself is test-covered. #3 met: MultiSource.CreateIn returns started per named queue, unit-tested with one capable and one incapable sub in the same composite; no real multi-queue fleet available for a live check. #4 met: ADR-0012 (widened Create, SetPhase-after-create, fold with TASK-22 all rejected with reasons) committed before the first adapter change (git log: d8b59d4 precedes b633ac4). CI bar met: build, vet, test, gofmt all clean; PR checks green. Not done: no --status flag (deliberate, per ADR); TASK-22 folding not pursued — its audit precondition is unmet.
<!-- SECTION:NOTES:END -->
