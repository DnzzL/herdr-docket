# Working with a software factory

[Back to the README](../README.md)

A factory turns well-stated work into merged code while you sleep. It does not
decide what is worth building, and it is only as good as the specs it is fed
and the corrections it keeps. The habits below are the difference.

**The grill is the intake.** An idea becomes work in one session with an
agent: it grills you until the idea is settled, then writes a spec and its
tickets under `.scratch/<slug>/` with `to-spec` and `to-tickets`, commits
them, and files one card that points at the spec
([from idea to card](how-it-works.md#from-idea-to-card)). The questions you
cannot answer in that session are the ones the factory would have guessed.

**The spec is the prompt.** The spec and every ticket in it answer three
things, and the third is the one that matters:

- **Outcome** — the problem in a user's words, not the fix you imagine.
- **Done when** — the observable change, one sentence.
- **Verify by** — how a stranger proves it: the test that is red today, the
  request and its expected response, the screen and what it should show.

If you cannot write *Verify by*, the work is not ready for an agent — it is
still a decision (see below). One spec, one card, one pull request; each
ticket a slice you would review in ten minutes. An idea whose pull request you
would not read in one sitting is two specs. A one-line fix needs no spec: a
card on its own, with its *Verify by*, worked as one ticket's worth of work.

| Instead of | Write |
| --- | --- |
| "Improve onboarding" | "New users drop at step 2. Done when: step 2 needs one field, not four. Verify by: the signup e2e completes with only an email." |
| "Fix the flaky test" | "`checkout.spec.ts` fails 1 run in 5 on CI. Done when: it waits on hydration, not a timeout. Verify by: 20 consecutive green runs, pasted." |
| "Refactor the planner" | Don't. Name the bug or the feature the refactor unblocks, and let the author choose the shape. |

**Delegate ideas, not solutions.** An idea goes in as a *spike*: "Investigate
X; deliverable: a note on the task with two or three options, their cost, and
a recommendation — no code." The author researches, you choose, and the
choice becomes the next spec, or a card, with its *Verify by*. Agents are good at options;
choosing between them is yours.

**What stays human** — and how the factory keeps it that way:

- **Product decisions**: what to build, for whom, what to drop. The grill
  settles them before a card exists; an author that meets one anyway blocks
  the card with the one question that would unblock it. Your `Blocked` column
  is your inbox: answer the question in a note, move the card back to the
  pickup column, done.
- **The irreversible**: schema and migrations, auth, payments, deploys,
  anything touching personal data. List those paths in the repo's
  `CODEOWNERS` and the gate holds every PR that touches them; label a single
  task `critical` for the same effect.
- **Taste and voice**: brand, copy, design direction. Give the factory a
  style guide in the repo and it will follow it; do not expect it to invent one.

**Correct the system, not the run.** When a merged PR is wrong, resist fixing
it by hand. File the fix as a card, and put the lesson where the next run
will read it: a line in the repo's `CLAUDE.md`, a lint rule, a test that
fails on the mistake. The second time you write the same review comment, it
should become a check. The repo is the factory's memory, and it compounds.

**Raise the dial slowly.** Start a queue with `verifier:` and no `merge:` —
read the verdicts, merge by hand. When a week of PASSes would have been your
merges too, add `merge: auto`, and add a `CODEOWNERS` line every time a merge
makes you nervous. `herdr-docket pause --now` stops everything at once;
`agent pause` parks one agent.

**A daily rhythm that works:**

- *Morning*: read the merge and hold notifications, answer the `Blocked`
  column, skim what merged (`herdr-docket history`).
- *During the day*: grill what you notice into specs; a small fix is a card
  with its *Verify by*. Ideas you cannot settle become spikes.
- *Evening*: leave the queue with a night's worth of verifiable specs.

**Acquisition and growth.** The factory ships; it does not find users. Split
the work the same way as product: positioning, channels, pricing and talking
to users are yours. What it does well is the measurable execution behind
them — landing-page variants, SEO pages from a keyword list, release notes
from merged PRs, analytics events, onboarding emails as code — each a ticket
whose *Verify by* is a number or an observable (the event fires, the page
scores 90 on Lighthouse, the email renders in the preview). Bring what your
users say — issues, support inbox, reviews — to the grill, and their
complaints become verifiable specs: that is the loop from users to code.

**What breaks a factory:** vague specs; specs too big for one pull request;
fixing agents' PRs by hand so nothing is learned; letting `Failed` pile up
unread; auto-merge on a repo with no CI; one model judging its own work.
