# Commands

[Back to the README](../README.md)

| | |
| --- | --- |
| `herdr-docket daemon` | the worker (Herdr starts it for you) |
| `herdr-docket init` | bootstrap the fleet dir |
| `herdr-docket auth basecamp` | sign in to a hosted queue, once |
| `herdr-docket auth github` | same, for a GitHub Projects board (`--token <pat>` to store one) |
| `herdr-docket auth notara --token ntr_…` | same, for a Notara database: stores the key, adds the fleet's statuses and agents as options |
| `herdr-docket list` | the queue, grouped by phase, with the routed agent |
| `herdr-docket run TASK-12` | run one task now |
| `herdr-docket task list` | the queue as the agent sees it (`--all` includes closed work) |
| `herdr-docket task view ID` | one task: body, notes, criteria, and who it is routed to |
| `herdr-docket task create "…" -a AGENT` | add work to the queue (`-s SOURCE` when several, repeatable `--ac "…"` for acceptance criteria) |
| `herdr-docket task assign ID AGENT` | hand a task to another agent, same id, same thread |
| `herdr-docket task note ID "…"` | say where things stand without closing |
| `herdr-docket task ask ID "ROUND"` | post the next grill round (`## Round N`) and park the card in `question` |
| `herdr-docket task answer ID "1.A 2.T …"` | post `## Answer N` for the latest round and send the card back to `triage` |
| `herdr-docket task move ID triage\|question\|todo` | file a card into one of the grill's states |
| `herdr-docket task body ID "…"` | replace a card's body — the triager writes the spec here |
| `herdr-docket task done\|fail\|block ID` | close with a verdict (`--note "…"` for the evidence; `--pr URL` delivers a PR to the verifier) |
| `herdr-docket task verdict ID PASS\|FAIL --pr URL --note "…"` | a verifier's verdict, pinned to the PR's diff; the note is the evidence it carries |
| `herdr-docket agent list` | the agents, and which are parked |
| `herdr-docket agent pause\|resume NAME` | park an agent, or unschedule nothing more for it |
| `herdr-docket runs` | the runs in flight; a record past its timeout is marked `stale?` |
| `herdr-docket status` | whether the daemon is alive: `daemon live · tick 3s ago`, `daemon down for 4 min` (no tick for 3 intervals), or `daemon never started` |
| `herdr-docket questions` | the daemon line, then the `question` cards across queues; pick one and answer its newest round question by question — Enter takes the `(reco)`, a letter that option, other text a free answer — then confirm to send it as `task answer` does. Fits 45 columns |
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
