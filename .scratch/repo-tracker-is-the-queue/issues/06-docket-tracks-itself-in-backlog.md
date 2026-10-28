# 06: docket tracks itself in its own backlog

**What to build:** this repo follows its own rule. `AGENTS.md` and `docs/agents/issue-tracker.md` point at `backlog/` (the template from 03), and `.scratch/` is deleted (every spec in it is resolved; git keeps the history).

**Blocked by:** 04

**Status:** ready-for-agent

- [ ] `AGENTS.md` names Backlog.md as the tracker
- [ ] `docs/agents/issue-tracker.md` is the backlogmd template
- [ ] `.scratch/` is gone

**Done when:** a skill asked to publish a spec in this repo writes a Backlog.md parent task.

**Verify by:** `test ! -d .scratch && grep -q "backlog task create" docs/agents/issue-tracker.md`.
