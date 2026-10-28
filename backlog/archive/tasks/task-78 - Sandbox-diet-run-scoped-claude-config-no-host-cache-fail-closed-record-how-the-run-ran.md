---
id: TASK-78
title: >-
  Sandbox diet: run-scoped claude config, no host cache, fail closed, record how
  the run ran
status: Blocked
assignee:
  - plugin-dev
created_date: '2026-10-18 16:56'
updated_date: '2026-10-18 17:55'
labels: []
dependencies: []
ordinal: 78000
---

## Description

<!-- SECTION:DESCRIPTION:BEGIN -->
Four changes on top of PR #49, all cheapest-first. (1) Stop binding the host ~/.claude read-write: the zsh shim creates a run-scoped config dir and exports it as CLAUDE_CONFIG_DIR (agent-sandbox expands "$VAR" in declared paths at launch), settings/hooks/skills stay read-only via the launcher's symlink resolution, auth is the env token only, session state dies with the run. (2) Point bun and npm caches at the sandbox's /tmp (BUN_INSTALL_CACHE_DIR, npm_config_cache) and drop rwDirs $HOME/.bun: a cache shared with unwalled processes is a persistence channel for a sandboxed agent. (3) Fail closed: today a sandbox path that is missing or lacks the rc chain makes the shim a no-op and the agent silently runs on the host; Provision must stat the path and fail with the reason. (4) Say how the run ran: the history line gains sandbox <path> or host and the prompt states it.
<!-- SECTION:DESCRIPTION:END -->

## Acceptance Criteria
<!-- AC:BEGIN -->
- [ ] #1 A sandbox path that does not exist, is not a directory, or lacks the rc chain fails the provision with the reason; no run reaches the host silently (test through the host ops seam).
- [ ] #2 The run history line and the run prompt each name sandbox <path> or host.
- [ ] #3 Inside the sandbox the host's ~/.claude/.credentials.json is unreadable, and bun install works with a sandbox-local cache (no write reaches the host cache).
<!-- AC:END -->

## Implementation Notes

<!-- SECTION:NOTES:BEGIN -->
Starting: worktree fast-forwarded onto PR #49 head (1b7f183). Reading host sandbox provision, shim, history/prompt seams before planning the four diet changes.

fleet: the agent settled without reporting a verdict.

fleet: the run's workspace wJ5 (pane wJ5:p1) is left open — jump in to resume.

Frozen 2026-10-18 with TASK-76: good idea, not now. Thaw: a decision from the human, nothing else.
<!-- SECTION:NOTES:END -->
