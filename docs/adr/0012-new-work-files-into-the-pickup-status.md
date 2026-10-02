# 0012 — New work files into the pickup status

Date: 2026-10-01 · Status: accepted · Decides TASK-40

## The problem the fleet hit

`herdr-docket task create` never names a status, so a backend files new work
into the project's own `default_status`. The fleet picks up exactly one
status per source — the mapped `todo` word. Where those two words differ
(Backlog.md reads only the mapped `todo` and `in_progress` words as open),
every task the fleet creates strands: open, named, and never claimed. The
intake meta-task, the reviewer's follow-ups, the lookback's pattern fixes
each stall at their first handoff, and no sweep names it because the tasks
look fine.

## The decision

Two moves, both inside the port's existing grammar:

1. **A fourth optional capability** beside `Phaser`, `Assigner` and
   `BaseBrancher`: `TodoStarter`, with one method,
   `CreateTodo(title, body, assignee) (string, error)`. A source that can
   start a task it creates in its own pickup status implements it; a source
   that cannot (a Basecamp list has no column to start a to-do in) does not,
   and says so by its absence. `Source.Create` keeps its three arguments.

2. The Backlog.md adapter implements it in **one backend write**: the Backlog
   CLI accepts a status at creation (`backlog task create --status`), so
   `CreateTodo` passes the project's own mapped `todo` word and no task ever
   exists in a column it does not belong in. There is no readback and no
   second round trip.

## What was rejected, and why

- **Widen `Create` with a starting status.** Every adapter — including those
  past this repo's reach — must then answer for an argument it can do
  nothing with. The port's smallest interface is the point of the port; this
  pays that for nothing a capability would not cover.
- **Create, then `SetPhase(Todo)` through the existing `Phaser`.** No port
  change, but `SetPhase` is display by contract — written best-effort, never
  read back, and "nothing about a phase decides what runs". A landing that
  decides whether the next tick can claim the task makes it load-bearing
  exactly where the contract forbids. It also pays a second backend round
  trip per create — a GitHub board points for each — and leaves a window
  where the task exists in the wrong column.
- **Fold with TASK-22** (one `Create` carrying status *and* criteria). The
  fold is only on the table "if the audit that task waits for has come
  back", and it has not: TASK-22 is parked, unstarted. No coupling was
  forced. If that task later widens creation to carry criteria, it decides
 its own shape against this one's record, not bound to it.

## The edges this leaves alone

A source without the capability creates as it did, and the CLI that reports
the say-so prints where the task landed rather than implying the fleet chose
it. The CLI gains no `--status` flag: a human who wants a specific column
writes it where it lives. MultiSource's `CreateIn` answers per queue, so one
queue's words decide only their own queue's landing column.
