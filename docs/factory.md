# The factory

[Back to the README](../README.md)

The loop the fleet runs when nobody is watching: feedback becomes tasks,
tasks become pull requests, pull requests become verdicts, and the week's
mistakes become next week's tasks. Everything on this page is built from two
things you already have — the queue and [herdr-automations](https://github.com/DnzzL/herdr-automations)
— plus the personas in [worked examples](examples.md). Nothing here adds a
service, an account, or a second daemon.

## The loop

```text
 sources ──► intake ──► queue ──► dev ──► PR ──► reviewer ──► merge or a human
    ▲                                                     │
    │                                                     ▼
    └────────────── lookback ◄── history ◄── runs ◄──────┘
                        │
                        └── one follow-up per pattern, back to the queue
```

| Stage | Persona (in [examples](examples.md)) | Schedule |
| --- | --- | --- |
| Turn feedback into verified tasks | [Example 5, `intake`](examples.md#5-an-intake-that-turns-feedback-into-fleet-work) | every few hours |
| Carry the task to a green PR | [Example 2, `dev`](examples.md#2-a-dev-that-works-ready-for-agent-tickets) | the queue, as ever |
| Verdict — and merge what the policy allows | [Example 4, `reviewer`](examples.md#4-a-reviewer-that-gates-the-devs-prs) | daily sweep |
| Nudge work that ended short | [Example 6, `stall`](examples.md#6-a-stall-sweep-that-nudges-work-that-ended-short) | twice a day |
| Find what keeps coming back | [Example 7, `lookback`](examples.md#7-a-lookback-that-finds-what-keeps-coming-back) | weekly |

The schedule never does the work. An automation's whole job is to put one task
on the queue — `herdr-docket task create … -a intake` — and the daemon runs it
on the persona like any other task, with `FLEET.md` and roles and budgets
intact. Automations decide *when*; the fleet decides *what* and *who*. That is
[the composition the README promises](../README.md#what-it-isnt), and this
page is where it gets a cron.

## The wiring

One `automations.yaml` for the whole loop. The prompts are deliberately thin —
they file tasks and nothing else:

```yaml
automations:
  # Delete "Report only." from the intake entry after you have read one
  # report and liked the gate. Until then nothing is filed.
  - name: intake
    cron: "15 */4 * * *"
    repo: ~/fleet
    workspace: root
    model: haiku
    prompt: |
      herdr-docket task create "Intake: turn new feedback into fleet tasks" -a intake \
        -d "Poll the sources in your persona's prompt — Sentry via mcp_config,
        gh issue list — and apply the gate. Report only."

  # The review sweep drives the reviewer persona, which lands paused because
  # only you know the repo it works. Point its workdir there, resume it, then
  # delete this line.
  - name: review-sweep
    disabled: true
    cron: "30 9 * * 1-5"
    repo: ~/fleet
    workspace: root
    model: sonnet
    prompt: |
      herdr-docket task create "Sweep: review the oldest un-reviewed PR" -a reviewer \
        -d "Review the oldest open pull request without a verdict, then re-task
        yourself for the rest. Your merge policy is in your persona."

  - name: stall-sweep
    cron: "0 13 * * *"
    repo: ~/fleet
    workspace: root
    model: haiku
    prompt: |
      herdr-docket task create "Sweep: nudge work that ended short" -a stall \
        -d "Read the queue and history.jsonl for the three stalls your persona
        names. Nudge with notes; decide nothing."

  - name: lookback
    cron: "0 10 * * 1"
    repo: ~/fleet
    workspace: root
    model: opus
    prompt: |
      herdr-docket task create "Lookback: what keeps coming back?" -a lookback \
        -d "Last 30 days against the 30 before. One follow-up per pattern,
        assigned to pm, evidence on each."
```

`repo:` is the fleet dir, because that is the checkout these tasks read;
`workspace: root` for the same reason the PM's example gives — the queue is the
files. `model:` is the video's lesson in one line: the pattern-matching goes to
the expensive model, the pollers to the cheap one.

Two knobs the loop leans on, both already built:

- **`mcp_config` on the agent** hands intake its Sentry or issue source — the
  persona never names a vendor, only the gate.
- **`model:` on the automation** that files the lookback is the cheap version
  of a second-model watchdog: a different model reading the same week's work.
  If you want real oversight, make it a stronger model than the one that did
  the merging.

## The dials

Nothing about how much autonomy the loop has lives in the loop. It is the
levers the [README's autonomy table](../README.md#how-much-autonomy) already
lists: which status the fleet may pick up, who takes unassigned work, where
`failed` points, the budgets per agent, `agent pause` on anything that
misbehaves. The loop runs *inside* those decisions — turn intake off by
pausing `intake`, slow the whole thing by giving `dev` two runs a day.

## Start small

The loop is one command, but it does not have to be run as one:

1. **File the tasks by hand first.** Create the intake task yourself for a
   week and read what the gate would file. Retune the persona between runs —
   it is a file.
2. **Add one schedule at a time.** Intake first, then the review sweep.
   Watch a few rounds of each on the board before the next enters the week.
3. **Leave the merge policy alone until the sweep has verdicts you have read
   yourself.** The policy paragraph in the reviewer persona is where the
   autonomy lives; loosen it one sentence at a time.

## One step

```bash
herdr-docket init --factory
```

Writes all six personas into an existing fleet dir — never overwriting one
you already have — appends the entries above when the automations plugin's
config is present, and points you at this page.

The three that read the queue (`intake`, `stall`, `lookback`) are pointed at
the fleet dir and run from the first tick. The three that work a repo (`pm`,
`dev`, `reviewer`) land **paused**, carrying the placeholder workdir from
[examples](examples.md): the loop refers to all three by name — intake files
its unclear items to `pm`, lookback files its patterns there, the reviewer
takes the dev's handoff — so a fleet missing them routes that work to nobody.
They exist, and they wait:

```bash
$EDITOR ~/fleet/agents/dev/AGENT.md     # workdir: your repo
herdr-docket agent resume dev
```

The `review-sweep` entry ships `disabled: true` for the same reason — a sweep
that files onto a paused reviewer is a pile, not a loop. Enable it once the
reviewer runs.

## What it refuses

- **The fleet never merges. Agents do**, under the merge policy written in
  the reviewer's own persona — a paragraph the one human owner edits. The
  daemon, the runner and the CLI have no forge verbs, and
  [ADR 0008](adr/0008-the-board-is-a-triage-surface.md) still holds: the
  board shows, it does not pass judgment.
- **No event triggers.** Work happens on a cron, because
  [that is all a local scheduler can honestly promise](https://github.com/DnzzL/herdr-automations/blob/main/docs/adr/0008-no-event-triggers.md).
  Polling a source every four hours is the same deal, said in cron.
- **Telemetry and repo layout are yours.** The loop's floor is the quality of
  what its sources emit and how verifiable its tasks are; that work is in
  your apps, not this plugin. A `history.jsonl` that grows forever is accepted
  for the same reason everything else here has no store: it is a file you can
  rotate.
- **The delivery record is the only thing the loop trusts on its own.** A
  worktree that ends `done` holding uncommitted work is kept, said on the
  task, and blocked to a human — the guard in `cleanup` is the one piece of
  the loop that is code rather than prose, and it exists because everything
  else here believes an agent's sentence.
