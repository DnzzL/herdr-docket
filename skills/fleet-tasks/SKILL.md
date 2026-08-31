---
name: fleet-tasks
description: Create and manage tasks in the fleet backlog — the shared Backlog.md queue that herdr-fleet routes to named agents. Use when asked to delegate work to a fleet agent, queue work for later, or report on fleet tasks.
---

# Fleet tasks

The fleet backlog is a Backlog.md project (default `~/fleet`, check
`herdr-fleet` output for the real path) that a daemon polls: every `To Do`
task assigned to a known agent gets picked up and run by that agent,
strictly one at a time.

All commands go through the `backlog` CLI **with `BACKLOG_CWD` pointing at
the fleet dir** — you are usually working in some other repo:

```bash
BACKLOG_CWD=~/fleet backlog task list --plain
BACKLOG_CWD=~/fleet backlog task create "Draft the launch post" \
  -d "What needs doing and why" --ac "one measurable criterion" -a dishnow-marketing
BACKLOG_CWD=~/fleet backlog task view TASK-12 --plain
```

Rules:

- **Two backlogs, two purposes.** The fleet backlog routes work to agents.
  A project's own `backlog/` (if it has one) tracks that project's dev work.
  A task for an agent goes in the fleet backlog; a plain dev todo goes in the
  project's. Never mix them up.
- **Assignee = agent.** `-a <name>` must match a folder in `<fleet>/agents/`.
  A task with no assignee is left alone (unless a default agent is configured).
- **Statuses are the lifecycle**: `To Do`, `In Progress`, `Blocked`, `Failed`,
  `Done`. Only the daemon and the task's own agent move a task; don't edit
  other tasks' statuses.
- **Creating a task is enough.** Don't try to start it — the daemon picks it
  up within seconds. `herdr-fleet run TASK-12` exists for humans who want it now.
- Give every task a real description and at least one acceptance criterion:
  the assigned agent gets exactly what the task says, nothing more.
