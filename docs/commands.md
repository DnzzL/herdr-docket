# Commands

[Back to the README](../README.md)

| | |
| --- | --- |
| `herdr-docket daemon` | the worker (Herdr starts it for you) |
| `herdr-docket init` | bootstrap the fleet dir |
| `herdr-docket init --factory` | …and install the factory loop (see [docs/factory.md](factory.md)) |
| `herdr-docket auth basecamp` | sign in to a hosted queue, once |
| `herdr-docket auth github` | same, for a GitHub Projects board (`--token <pat>` to store one) |
| `herdr-docket list` | the queue, grouped by phase, with the routed agent |
| `herdr-docket run TASK-12` | run one task now |
| `herdr-docket task list` | the queue as the agent sees it (`--all` includes closed work) |
| `herdr-docket task view ID` | one task: body, notes, criteria, and who it is routed to |
| `herdr-docket task create "…" -a AGENT` | add work to the queue (`-s SOURCE` when several, repeatable `--ac "…"` for acceptance criteria) |
| `herdr-docket task assign ID AGENT` | hand a task to another agent, same id, same thread |
| `herdr-docket task note ID "…"` | say where things stand without closing |
| `herdr-docket task done\|fail\|block ID` | close with a verdict (`--note "…"` for the evidence; `--pr URL` delivers a PR to the verifier) |
| `herdr-docket task verdict ID PASS\|FAIL --pr URL` | a verifier's verdict, pinned to the PR's diff |
| `herdr-docket agent list` | the agents, and which are parked |
| `herdr-docket agent pause\|resume NAME` | park an agent, or unschedule nothing more for it |
| `herdr-docket runs` | the runs in flight; a record past its timeout is marked `stale?` |
| `herdr-docket stop TASK-12` | stop a task's run in flight |
| `herdr-docket pause [--now]` / `resume` | pause the whole fleet — no new runs, no pipeline stages (`--now` stops runs in flight) |
| `herdr-docket pane` | the runs overlay (the plugin opens it for you) |
| `herdr-docket history [TASK-12]` | recent runs: how long, the verdict, what it produced — branch, commits, PR |
| `herdr-docket logs` | the daemon log's tail (`-n LINES`), without guessing where it lives |
| `herdr-docket install-skill` | teach your coding agent to write fleet tasks |

## Teaching your agents

`skills/fleet-tasks/SKILL.md` teaches a coding agent to create well-formed fleet
tasks — real descriptions, at least one acceptance criterion, and the rule that
keeps the fleet queue separate from a project's own. Agents only discover skills
under `~/.claude/skills`, so install it once:

```bash
herdr-docket install-skill     # symlinks into ~/.claude/skills
```

It points a symlink at the bundled skill, so plugin upgrades update the skill
too. Start a new agent session afterwards.
