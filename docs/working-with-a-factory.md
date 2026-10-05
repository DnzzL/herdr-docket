# Working with a software factory

[Back to the README](../README.md)

A factory turns well-stated work into merged code while you sleep. It does not
decide what is worth building, and it is only as good as the tasks it is fed
and the corrections it keeps. The habits below are the difference.

**The task is the prompt.** Every task answers three things, and the third is
the one that matters:

- **Outcome** — the problem in a user's words, not the fix you imagine.
- **Done when** — the observable change, one sentence.
- **Verify by** — how a stranger proves it: the test that is red today, the
  request and its expected response, the screen and what it should show.

If you cannot write *Verify by*, the task is not ready for an agent — it is
still a decision (see below). One task, one pull request, a few hours of
work: a task you would not review in ten minutes is two tasks.

| Instead of | Write |
| --- | --- |
| "Improve onboarding" | "New users drop at step 2. Done when: step 2 needs one field, not four. Verify by: the signup e2e completes with only an email." |
| "Fix the flaky test" | "`checkout.spec.ts` fails 1 run in 5 on CI. Done when: it waits on hydration, not a timeout. Verify by: 20 consecutive green runs, pasted." |
| "Refactor the planner" | Don't. Name the bug or the feature the refactor unblocks, and let the author choose the shape. |

**Delegate ideas, not solutions.** An idea goes in as a *spike*: "Investigate
X; deliverable: a note on the task with two or three options, their cost, and
a recommendation — no code." The author researches, you choose, and the
choice becomes the next task with its *Verify by*. Agents are good at options;
choosing between them is yours.

**What stays human** — and how the factory keeps it that way:

- **Product decisions**: what to build, for whom, what to drop. Intake files
  anything it cannot verify straight into your `Blocked` column with the one
  question that would unblock it. That column is your inbox: answer the
  question in a note, move the task back to the pickup column, done.
- **The irreversible**: schema and migrations, auth, payments, deploys,
  anything touching personal data. List those paths in the repo's
  `CODEOWNERS` and the gate holds every PR that touches them; label a single
  task `critical` for the same effect.
- **Taste and voice**: brand, copy, design direction. Give the factory a
  style guide in the repo and it will follow it; do not expect it to invent one.

**Correct the system, not the run.** When a merged PR is wrong, resist fixing
it by hand. File the fix as a task, and put the lesson where the next run
will read it: a line in the repo's `CLAUDE.md`, a lint rule, a test that
fails on the mistake. The second time you write the same review comment, it
should become a check. The weekly lookback files these patterns for you; the
repo is the factory's memory, and it compounds.

**Raise the dial slowly.** Start a queue with `verifier:` and no `merge:` —
read the verdicts, merge by hand. When a week of PASSes would have been your
merges too, add `merge: auto`, and add a `CODEOWNERS` line every time a merge
makes you nervous. `herdr-docket pause --now` stops everything at once;
`agent pause` parks one agent.

**A daily rhythm that works:**

- *Morning*: read the merge and hold notifications, answer the `Blocked`
  column, skim what merged (`herdr-docket history`).
- *During the day*: turn what you notice into tasks — two minutes each, with
  *Verify by*. Ideas become spikes.
- *Evening*: leave the queue with a night's worth of small, verifiable work.

**Acquisition and growth.** The factory ships; it does not find users. Split
the work the same way as product: positioning, channels, pricing and talking
to users are yours. What it does well is the measurable execution behind
them — landing-page variants, SEO pages from a keyword list, release notes
from merged PRs, analytics events, onboarding emails as code — each a task
whose *Verify by* is a number or an observable (the event fires, the page
scores 90 on Lighthouse, the email renders in the preview). Point intake at
where your users already talk — issues, support inbox, reviews — and their
complaints arrive as verifiable tasks: that is the loop from users to code.

**What breaks a factory:** vague tasks; big tasks; fixing agents' PRs by hand
so nothing is learned; letting `Failed` pile up unread; auto-merge on a repo
with no CI; one model judging its own work.
