---
name: fleet-tasks
description: Create and manage tasks in the fleet queue — the shared task list that herdr-fleet routes to named agents. Use when asked to delegate work to a fleet agent, queue work for later, or report on fleet tasks.
---

# Fleet tasks

The fleet works one shared queue: every open task assigned to a known agent is
picked up by that agent and run. Agents work in parallel, but each agent runs
one task at a time.

Everything goes through the `herdr-fleet` CLI — you do not need to know, or
care, where the queue physically lives:

```bash
herdr-fleet task list                       # open work, grouped by phase
herdr-fleet task list --all                 # include closed work
herdr-fleet task view TASK-12               # description, criteria, notes
herdr-fleet task create "Draft the launch post" \
  -d "What needs doing and why" -a dishnow-marketing
herdr-fleet task note TASK-12 "halfway; blocked on the API key"
herdr-fleet task done TASK-12 --note "shipped; criterion 1 met, 2 dropped"
herdr-fleet task fail TASK-12 --note "could not reproduce the crash"
herdr-fleet task block TASK-12 --note "needs a decision on scope"
```

Rules:

- **Two queues, two purposes.** The fleet queue routes work to agents. A
  project's own `backlog/` (if it has one) tracks that project's dev work. A
  task for an agent goes in the fleet queue; a plain dev todo goes in the
  project's. Never mix them up.
- **Assignee = agent.** `-a <name>` must match a folder in `<fleet>/agents/`.
  A task with no assignee is left alone, unless a default agent is configured
  — and a task assigned to a name nobody has is left alone too, so check
  `herdr-fleet list` if work seems stuck.
- **Say what "done" means.** The task description is the whole brief: the
  agent gets exactly what the task says, nothing more. Acceptance criteria are
  added by whoever owns the queue (`-d` is the field `herdr-fleet` writes), and
  the agent reports on them in prose rather than ticking boxes.
- **Creating a task is enough.** Don't try to start it — the daemon picks it
  up within seconds. `herdr-fleet run TASK-12` exists for humans who want it now.
- **Closing is the report.** `done`, `fail` and `block` all take a `--note`;
  the note is where the reasoning goes. A task left open will be run again, so
  always close what you touch.
