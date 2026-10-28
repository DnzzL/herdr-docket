# 02: The pick skips a ticket

**What to build:** `work.Task` gains `Parent string` (the parent task's id, empty for a top-level task). Backlog.md reads it from the task's `parent_task_id` (`task view --plain` or the list output); GitHub reads `parent { number }` on the issue. The pick never offers a task with a Parent: a ticket is the implementer's plan, not a card. Nothing writes Parent.

**Blocked by:** None (can start immediately)

**Status:** resolved

- [x] `work.Task.Parent` exists, documented as "a ticket of the spec named here; never picked"
- [x] backlogmd fills Parent for a subtask created with `backlog task create --parent` (test against the real CLI, as the adapter's tests do)
- [x] github fills Parent from `parent { number }` (fake board test)
- [x] A pick over a To Do parent and two To Do subtasks, all assigned, returns only the parent

**Done when:** a subtask is never routed to an agent.

**Verify by:** `go test ./internal/pick/ ./internal/work/...` green; the pick test is red before the change.
