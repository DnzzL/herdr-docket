# Commands

[Back to the README](../README.md)

| | |
| --- | --- |
| `herdr-docket daemon` | the worker (Herdr starts it for you) |
| `herdr-docket init` | bootstrap the fleet dir: agents, roles, `FLEET.md` — no queue |
| `herdr-docket init REPO` | make a project repo the queue: Backlog.md with the fleet's statuses (`backlog init` also writes its guidelines into the repo's `AGENTS.md`), `docs/agents/issue-tracker.md`, the repo as a source in `fleet.yaml` — beside any queue it already names |
| `herdr-docket init REPO --github OWNER/N` | the same for a repo tracked on GitHub project board N: a GitHub `docs/agents/issue-tracker.md` and a `kind: github` source (`repo` read from `origin`), no Backlog.md |
| `herdr-docket auth github` | sign in to a GitHub Projects board, once (`--token <pat>` to store one) |
| `herdr-docket list` | the queue, grouped by phase, with the routed agent |
| `herdr-docket run TASK-12` | run one task now |
| `herdr-docket task list` | the queue as the agent sees it (`--all` includes closed work) |
| `herdr-docket task view ID` | one task: body, notes, criteria, and who it is routed to |
| `herdr-docket task create "…" -a AGENT` | add work to the queue (`-s SOURCE` when several, repeatable `--ac "…"` for acceptance criteria); with no `-a` on a queue that names a `triager`, the card is created straight in `triage`. Prints the status it landed in |
| `herdr-docket task assign ID AGENT` | hand a task to another agent, same id, same thread |
| `herdr-docket task note ID "…"` | say where things stand without closing |
| `herdr-docket task ask ID` | park a card in `question` once its `## Round N` note is posted |
| `herdr-docket task answer ID "1.A 2.T 3. …"` | answer the card's newest round (`## Answer N` note) and send it back to `triage`; refuses a card not in `question`, or one a run still holds (retry once it settles) |
| `herdr-docket task done\|fail\|block ID` | close with a verdict (`--note "…"` for the evidence; `--pr URL` delivers a PR to the verifier) |
| `herdr-docket task verdict ID PASS\|FAIL --pr URL --note "…"` | a verifier's verdict, pinned to the PR's diff; the note is the evidence it carries |
| `herdr-docket queues` | the queue names, one per line, for scripts; nothing for a single unnamed `source:`, which takes no `-s` |
| `herdr-docket agent list` | the agents, and which are parked |
| `herdr-docket agent pause\|resume NAME` | park an agent, or unschedule nothing more for it |
| `herdr-docket runs` | the runs in flight; a record past its timeout is marked `stale?` |
| `herdr-docket status` | whether the daemon is alive: `daemon live · tick 3s ago`, `daemon down for 4 min` (no tick for 3 intervals), `daemon never started`, or `daemon unknown · heartbeat unreadable` |
| `herdr-docket questions` | on a terminal, the pane's Round screen: straight on the round when one card waits, on the pane's Home when several do; tap or click the options, then Send, which posts as `task answer` does. Piped (no terminal), the line prompt: the daemon line, the `question` cards across queues; pick one and answer its newest round question by question — Enter takes the `(reco)`, a letter that option, other text a free answer — then confirm to send. Fits 45 columns |
| `herdr-docket stop TASK-12` | stop a task's run in flight |
| `herdr-docket pause [--now]` / `resume` | pause the whole fleet — no new runs, no pipeline stages (`--now` stops runs in flight) |
| `herdr-docket pane` | the overlay (the plugin opens it for you): the daemon line, Your turn (questions, Blocked cards, merges held for you), runs in flight, the last 5 settled; cut to the terminal width. A question card opens its Round, answered by taps and Send |
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
