# herdr-docket

**Give a task to a named coding agent. It opens a pull request, a second agent
verifies it, and a merge gate written in code merges it — or hands it to you.**

A task queue and a small software factory for [Herdr](https://herdr.dev): your
agents work the queue in parallel, in workspaces you can watch, join or close.

[![CI](https://github.com/DnzzL/herdr-docket/actions/workflows/ci.yml/badge.svg)](https://github.com/DnzzL/herdr-docket/actions/workflows/ci.yml)
[![Release](https://img.shields.io/github/v/release/DnzzL/herdr-docket)](https://github.com/DnzzL/herdr-docket/releases)
[![License](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)

```text
 you / intake ──► task ──► author ──► PR ──► verifier ──► gate ──► merged
 (says how it's                ▲               │ FAIL       │
  verified)                    └───────────────┘            └──► held: your Blocked column
```

## Who it's for

- You use **Herdr** and run more than one coding agent.
- You spend your time **babysitting** them: re-prompting, checking whether they
  finished, reviewing PRs nobody verified.
- You want autonomy you can **turn up one project at a time**, not a switch
  that hands over your repo.

## Quick start

Requires Herdr ≥ 0.8 (Linux or macOS), an agent CLI Herdr can start — Claude
Code by default; pi, Codex, Gemini, opencode and [others](docs/agents.md#running-a-post-on-another-agent-runtime)
work too — and `gh` for pull requests.

```bash
herdr plugin install DnzzL/herdr-docket
herdr-docket init                          # a task queue + an example agent in ~/fleet
$EDITOR ~/fleet/agents/example/AGENT.md     # point it at your repo
herdr-docket task create "Search ignores accents" -a example \
  -d "Users who type « creme » find none of the « crème » products.
Done when: a query matches with or without accents.
Verify by: a search test for « creme » that is red on main, green after."
```

Within 15 seconds Herdr opens a workspace on the agent's repo and starts the
agent with its persona and the task. It reports back through the CLI, closes
the task with a verdict, and the run lands in `herdr-docket history`.

## How it works

1. **A task** says what to do and how a stranger proves it is done.
2. **The daemon** routes it to the agent it names and starts a run in a Herdr
   workspace — one run per agent, agents in parallel.
3. **The author** delivers a pull request.
4. **A verifier** that did not write it re-derives every claim and records
   PASS or FAIL. A FAIL goes back to the author, twice at most.
5. **The gate** — code, not a prompt — merges a PASS on green CI, unless the
   task is `critical` or the diff touches your `CODEOWNERS`. Anything else
   waits in your `Blocked` column with the one reason it stopped.

More: [how it works](docs/how-it-works.md) · [the factory loop](docs/factory.md).

## How much autonomy

Start with a hand on it, then let go one project at a time:

| Step | `fleet.yaml` | You do |
| --- | --- | --- |
| 1. Agents work, you review | a queue, an agent | read and merge every PR |
| 2. A verifier reviews first | `verifier: reviewer` | merge what passed |
| 3. The gate merges | `merge: auto` + a `CODEOWNERS` | answer the `Blocked` column |

The fleet only ever picks up the status you name, so your triage and wontfix
columns stay yours. Every lever: [how it works](docs/how-it-works.md#the-levers).

## Features

- **Your queue, where it already lives** — a [Backlog.md](https://backlog.md)
  project in git, Basecamp, or a GitHub Projects board, and several at once
  ([where the queue lives](docs/queues.md)).
- **Agents are markdown** — one `AGENT.md` each: run parameters plus a persona
  ([writing an agent](docs/agents.md), [worked examples](docs/examples.md)).
- **The whole factory in one command** — `herdr-docket init --factory` adds
  intake, verifier, a stall sweep and a weekly lookback
  ([the factory](docs/factory.md)).
- **A brake** — `herdr-docket pause` stops new runs fleet-wide; the **runs**
  pane shows what is in flight and stops a run with one key.
- **No service, no store** — markdown in git and one JSONL history file.

## What it isn't

- **Not a scheduler.** Recurring work belongs to
  [herdr-automations](https://github.com/DnzzL/herdr-automations): automations
  decide *when*, the fleet decides *what* and *who*.
- **Not a workflow engine.** A task is one goal; its only fixed sequence is
  author → verifier → gate. Agents fan out by filing follow-up tasks.
- **Not an autonomous company.** [paperclip.ing](https://paperclip.ing) runs
  one; this is the smallest version of the idea that still works — named
  agents, a shared queue, no org chart, autonomy as a dial.

## Docs

[How it works](docs/how-it-works.md) ·
[working with a factory](docs/working-with-a-factory.md) ·
[the factory loop](docs/factory.md) ·
[writing an agent](docs/agents.md) ·
[worked examples](docs/examples.md) ·
[where the queue lives](docs/queues.md) ·
[commands](docs/commands.md)

## Contributing

Issues and pull requests are welcome — a new queue adapter especially.
Build, test and house style: [CONTRIBUTING.md](CONTRIBUTING.md).

## License

MIT.
