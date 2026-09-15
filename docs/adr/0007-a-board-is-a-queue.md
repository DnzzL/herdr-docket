# A board is a queue: issues on it, and a page of them

GitHub Projects v2 is the third backend behind `work.Source`. It does not fit
the port the way the other two do, so the three decisions that shaped it are
written down here. The rule that settles all three: **the board holds what an
issue has no concept of — an order, a column, and the routing key — and the
issue holds everything else.**

**A task is a real issue that has been put on the board.** Not one of the
board's own draft cards, which are cheaper and would have been the obvious
choice: a draft card cannot be commented on, and work that leaves no trace is
not work a fleet can hand on. So the board is the queue and the repository is
the container: `List` reads the board, `Create` writes an issue into the
repository *and* onto the board in one mutation, and an issue in the repository
that is not on the board is not the fleet's work. Saying so — rather than
adding it — is what keeps a fleet from claiming a list it was never given.

**Open is the issue's own state, and a verdict is a comment.** `List` reads
`state`; a task is open while the issue is. The column is display only, and is
believed only where it names an ending, because the board cannot say *how* a
task ended: GitHub's vocabulary is `COMPLETED` or `NOT_PLANNED`, which cannot
tell failed from blocked. `Close` closes the issue with the reason the verdict
implies and writes `Verdict: <word>` to the thread, in one document, close
first — so a failure to comment leaves an issue the fleet has finished with but
whose outcome is honestly unknown, rather than one still open and about to run
again. `Get` reads the verdict back from the **newest** such comment, and only
from a closed issue: an open one still carrying "Verdict: blocked" is work a
human has since unblocked, and the phase must not follow the stale word.

**`List` is one page of 100, and the poll pays for nothing else.** Points are
charged per hundred objects returned on GitHub's GraphQL API (5,000 per hour),
so a body or a comment stream per item would turn a five-second poll from about
one point into about a hundred. The page asks for titles, states, timestamps
and the two columns, in board order; bodies, checklists and threads are read by
`Get`, which a person calls. Past 100 items the tail is dropped, exactly as
[ADR 0002](0002-list-is-a-page-and-a-verdict-costs-a-request.md) accepts for
every backend.

**Status:** accepted

## Considered options

- **Draft issues instead of real issues.** Rejected: no comments, so no notes,
  no verdict and no thread — a task an agent can move but never talk about.
  `convertProjectV2DraftIssueItemToIssue` exists, and converting a card on the
  way in is a write the fleet would have to make for every task it created.
- **A label, or an issue's assignee, as the routing key.** Rejected: labels are
  repository-wide, so routing a board would mean writing labels into a
  repository that may have its own convention for them; the GitHub *assignee*
  is a GitHub user, and an agent is not one.
- **Read the column back as the phase, so the board's own workflow decides.**
  Rejected: it hands a drag of the mouse the power to close work, or to keep a
  finished task in the queue. The column is a description of the issue's state,
  not a second source of truth about it.
- **A different comment per ending, no `Verdict:` prefix.** Rejected: Basecamp
  already writes `Verdict: <word>`, the word is read by people as often as by
  the fleet, and two adapters disagreeing about it would be two dialects for
  one idea.
- **Filter the queue client-side instead of pushing a search at the server.**
  Rejected: a page of 100 is taken from the board's own order, so filtering
  after the fact would return fewer tasks than a page for no reason, and the
  filter the server applies is the one a user could type into the board's
  search box.
- **Add a task to the board lazily in `Assign`, or when a run starts.**
  Rejected: `Create` puts an issue on the board in the same mutation that makes
  it, so there is no orphan to heal — and a fleet that added items as it went
  would be writing to a board a human is editing.

## Consequences

- **A pull request or a draft card on the board is invisible to the fleet.**
  The queue's filter is `is:issue`, and the document asks only for the fields
  an `Issue` has. A bot that opens a PR into the board's repository is not a
  task, which is the intended reading and worth knowing.
- **The routing key is a board field, so it has to exist before a task can be
  routed.** `auth github` provisions it — one option per `agents/<name>/` — and
  every write refuses an agent that is not an option rather than clearing the
  field, because clearing it takes the task off every agent's board at once and
  looks like it worked. Provisioning never removes an option: a single-select
  field's options are written as a whole set, and an option that disappears
  takes every item's value with it.
- **A token has no expiry the fleet can see**, so the only notice that one was
  revoked is a 401. That empties the memo and the next call resolves again —
  environment, then `gh`, then the stored file. A token changed on disk is
  picked up only after such a failure, which is the price of not running `gh`
  on every tick.
- **A close that half-applies is safe in the direction it can go wrong.**
  GitHub runs a document's mutations in order but reports each on its own, so
  `closeIssue` can fail while the comment lands. What is left is an issue that
  is still open carrying `Verdict: done` — and the fleet reads a verdict only
  from a closed issue, so the task stays in the queue and runs again rather
  than being filed as finished on the strength of a comment.
- **Signing in is the one write to a board's schema**, and it is deliberate:
  a fleet that added an option mid-run would be rewriting a board a person is
  editing at the same time.
- **A GitHub Enterprise host is not supported.** `api.github.com` is spelled
  once, in `GraphQLURL`, and a second host is a configuration field rather than
  a rewrite — but it is not one yet.
