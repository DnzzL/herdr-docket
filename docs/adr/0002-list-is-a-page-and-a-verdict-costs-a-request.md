# List is a page; a verdict costs a request

`List` is the port's *poll*: the board calls it every five seconds and the
daemon every fifteen. So it returns one page — whatever the backend hands back
by default — and carries no cursor. At the fleet's own scale (a personal
queue) a page holds the whole list; past that, the tail is dropped, and that
is accepted rather than paid for at the port. A *verdict* is the opposite kind
of read: a binary backend records it as a comment, so reading it costs one
request **per task**. The rule that settles both: pay per-task requests only
where a single task is already being read for a decision that needs it, never
on the poll.

**Status:** accepted

## Considered options

- **Give `List` a cursor.** Rejected: it is interface-level — every adapter,
  every caller and the contract suite grow a pagination loop — to buy
  something a personal queue does not have (a list too long for one page). A
  limit written down is cheaper than a protocol nobody needs yet.
- **Paginate inside the adapter, leaving the port cursoless.** Rejected: it
  makes one `List` call unbounded — follow every `Link` — so the board's
  five-second poll could fetch a thousand closed to-dos. A cursoless `List`
  that quietly pages is worse than one that truncates, because its cost stays
  invisible until it is a rate-limit.
- **Read the verdict back in `List`, so the board groups by it.** Rejected:
  one request per task per poll, for a display word. `Get` already spends the
  request, so it sets the phase from the verdict there; `List` does not, and
  the board stands a closed binary-backend task under its phase word.
- **Never read the verdict back at all.** Rejected: the runner decides whether
  to tear down a successful run's workspace from the verdict. With none, every
  Basecamp run — including the ones that finished — would look unfinished and
  keep its workspace open.

## Consequences

- A Basecamp `List` silently drops work past the API's default page (about 50
  to-dos per list). It is invisible at personal scale, and this ADR is where
  the cause is written down for the day a page-long list makes it visible.
- `Get` returns a closed to-do whose phase is the verdict's label when one was
  recorded, so `herdr-docket task view` shows `Failed` where the board shows
  `Done`. A hand-ticked to-do has no verdict and stays under `Done`, which is
  all Basecamp's completed flag actually says.
- A third backend's obligations are unchanged: `List` is one page, and `Get`
  reads the verdict where that backend records one.
- If the fleet ever outgrows a page, the fix is a cursor on `List` and this
  ADR is superseded — not rediscovered.
