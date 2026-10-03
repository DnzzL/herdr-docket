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
| Nudge work that ended short | [Example 6, `stall`](examples.md#6-a-stall-sweep-that-nudges-work-that-ended-short) | daily |
| Find what keeps coming back | [Example 7, `lookback`](examples.md#7-a-lookback-that-finds-what-keeps-coming-back) | weekly |

The schedule never does the work. An automation's whole job is to put one task
on the queue — `herdr-docket task create … -a intake` — and the daemon runs it
on the persona like any other task, with `FLEET.md` and roles
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
        -d "Poll the sources your persona and mcp_config name and apply the
        gate. Report only."

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
        -d "Review the oldest open pull request whose head commit carries no
        verdict of yours. A PR you have already judged stays covered while it
        waits on a human, and comes back to you only when new commits land on
        it. Then re-task yourself for the rest. Your merge policy is in your
        persona."

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
        assigned to dev, evidence on each."
```

`repo:` is the fleet dir, because that is the checkout these tasks read;
`workspace: root` for the same reason the PM's example gives — the queue is the
files. `model:` is the video's lesson in one line: the pattern-matching goes to
the expensive model, the pollers to the cheap one.

Two knobs the loop leans on, both already built:

- **`mcp_config` on the agent** hands intake its feedback source — an error
  tracker, an issue tracker, support mail — the persona never names a vendor,
  only the gate.
- **`model:` on the automation** that files the lookback is the cheap version
  of a second-model watchdog: a different model reading the same week's work.
  If you want real oversight, make it a stronger model than the one that did
  the merging.

## The dials

Nothing about how much autonomy the loop has lives in the loop. It is the
levers the [README's autonomy table](../README.md#how-much-autonomy) already
lists: which status the fleet may pick up, who takes unassigned work, where
`failed` points, `agent pause` on anything that
misbehaves. The loop runs *inside* those decisions — turn intake off by
pausing `intake`.

## Start small

The loop is one command, but it does not have to be run as one:

1. **File the tasks by hand first.** Create the intake task yourself for a
   week and read what the gate would file. Retune the persona between runs —
   it is a file.
2. **Add one schedule at a time.** Intake first, then the review sweep.
   Watch a few rounds of each in `herdr-docket history` before the next enters the week.
3. **Leave the merge policy alone until the sweep has verdicts you have read
   yourself.** The policy paragraph in the reviewer persona is where the
   autonomy lives; loosen it one sentence at a time.

## One step

```bash
herdr-docket init --factory
```

Writes all five personas into an existing fleet dir — never overwriting one
you already have — appends the entries above when the automations plugin's
config is present, and points you at this page.

The three that read the queue (`intake`, `stall`, `lookback`) are pointed at
the fleet dir and run from the first tick. The two that work a repo (`dev`,
`reviewer`) land **paused**, carrying the placeholder workdir from
[examples](examples.md): the loop refers to both by name — lookback files its
patterns to `dev`, the reviewer takes the dev's handoff — so a fleet missing
them routes that work to nobody. Intake files what it cannot verify as a
blocked task: that column is yours.
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

## The first week — 2026-09-25 to 2026-10-02

The loop's first full week ran on the DnzzL fleet over DishNow-v2 and
herdr-docket themselves. What follows is the record, kept here because the
next person tuning their dials deserves the evidence, not the pitch.

**The clock held.** Eight days of cron, no human ever starting a run:
intake 8:15 and 16:15, the review sweep 9:30 every day, the stall sweep
13:00 daily, lookback Mondays 10:00, pm-triage weekday mornings.
Sixty-eight automations runs in the window — ninety-one all-time on the
automations books, every failure or invalidation on those books dated
2026-09-15, pre-factory — and not one failure in the window. Every persona
filed through the queue CLI and every run the daemon picked worked the
task like any other.

**Every merge that happened had a reviewer verdict behind it.** Eight
DishNow pull requests merged in the window. Five were merged by the
reviewer persona itself inside its merge gate (#36, #37, #39, #41, #42);
three (#35, #38, #40) were approved by the reviewer and merged by the human,
which is what the policy prescribes when the diff is beyond the gate — a
data migration, an API-boundary change, a 32-file refactor. What one week
cannot settle is where the press settles as the gate loosens: the persona
pressed merge five times to the human's three — the opposite of a
mostly-human loop, with the gate deliberately narrow in week one. That is
a dial, and it turned itself somewhere in week one.

**The reporting half of the loop broke, and was fixed inside the week.**
Twenty-one runs in a fortnight did the work — some pushed green pull
requests — then settled without reporting a verdict (26 failed history
records — some runs recorded twice), so the queue recorded failure for
delivered work. The lookback filed the pattern
as [docket/TASK-49](../backlog/tasks/) from the history counts, the fix
shipped as v0.8.0's prompt change (the commands in a prompt must resolve,
so they are printed absolute), and zero recurrences in the day after. The
Monday lookback closes it with its own counts.

**What the week left stranded** (the queue carries each; the doc records
the shape):

- **A Blocked task got a run.** On 2026-10-02 at 10:14 a reviewer run
  started on fleet/TASK-1, a task parked waiting on a human — the run left
  no terminal record in history.jsonl and its workspace pane is still
  open. Which code path let the pick through is disputed: today's `open()`
  in `internal/work/backlogmd` is a whitelist (a Blocked task is not open
  by it), the repo pick tests hold a Blocked task unpicked, and the
  incident's own history rows are of unverified provenance. Suspended
  behind [docket/TASK-53](../backlog/tasks/) until its red/green proof
  says which is true — filed as a defect, not a dial: waiting on a human
  is not work an agent can spend a run on.
- **A verdict can be reached and still recorded failed** (the docket/TASK-46
  review run posted its review and filed its follow-up, then reported
  failed). The same TASK-49 pattern, on the done/failed side rather than the
  silence side.
- **A rebase rewrites someone's work, and the fleet does not.** DishNow
  PR #43's second commit is under the fleet persona's local identity
  (`fleet <fleet@herdr.local>`, no human name); a rebase rewrites what the
  fleet's own run wrote there, and that is a human decision the reviewer
  correctly refused to make alone.
- **A cross-branch duplicate task id makes queue polls warn.** Each worktree
  branch carries the queue files it was cut from, so `backlog task list`
  sees two task-45s until the branches merge. Diagnostic only in the
  routing sense — the fleet's other sources kept routing — but the docket
  source's own polls failed on it repeatedly (daemon.log 2026-10-02
  12:16–12:25, `queue poll failed: source "docket"` every fifteen
  seconds), so the sweeps that read that queue mid-week have to read past
  the duplicate's warning.

### Known gaps

- The Monday lookback is the loop's memory and its one scheduled run of
  the week ran twenty seconds by scheduled→done timestamps (10:00:18 →
  10:00:38 on 2026-09-28; the history carries no duration field) and filed
  nothing; the pattern follow-up that exists
  (docket/TASK-49) came from a demo lookback run instead. Why the scheduled
  one under-performed is unanswered.
- One day of post-fix evidence is not a pattern's death: docket/TASK-49
  stays open until the lookback closes it with counts.
- The merge press's week-one split — five persona presses to three human —
  is one week of data, not a resting point. Nothing in the loop forces it to
  move; the dial is the persona's merge policy, loosened one sentence at a
  time.

## Known gaps

- `not done` — the worker → verifier → merge-gate pipeline of
  [ADR 0013](adr/0013-the-factory-is-a-worker-a-verifier-and-a-gate-in-code.md):
  the reviewer still merges on its own judgment, from its persona's prose.
- `fragile` — nothing caps a self-tasking loop since budgets went; `agent
  pause` is the brake, applied by a human reading `history`.
