# The timeout records a late run, it does not drop the delivery

The failure this answers: on 2026-10-04 nine DishNow runs ended `failed:
timed out waiting for agent status` at exactly their agent's
`timeout_minutes` (dev 90 minutes, reviewer 45). The timeout closed the
task Failed but never stopped the agent — it kept working, then opened its
PR or wrote its verdict into a queue that had stopped listening.
`history.jsonl` holds dishnow/TASK-143's run as both `status:failed` and
`verification:PASS`; TASK-145's dev pane worked 1h40 past its deadline
before delivering PR #61; five PRs never met a verifier and two PASSes were
merged by hand. The timeout was a silent drop: the delivery existed, and
nothing picked it up.

**The pick: the run keeps the agent it started.** The ticket offered two
ways out — resume the pipeline when that agent settles, or end the agent at
the deadline so nothing is delivered after the verdict. Ending the agent
destroys exactly what the fleet exists to collect: the fleet had already
watched five late PRs be real work, and the verifier and the gate don't
judge a delivery's punctuality, they judge its diff. Killing at the
deadline converts every overrun into lost work; keeping the agent converts
it into late work. The second option survives only as the tail of the first
— see the bound below.

**How the deadline travels: as a fact, behind the host port.** `host.Do`
still returns at the deadline — it answers the new `host.ErrTimedOut`, and
`host.Settle(session, window)` is the same wait with nothing started and
nothing submitted: the prompt is already in front of the agent. The runner
records `timed_out` (a mark, not an ending — only done, failed and
cancelled close a run), keeps the workspace, and listens. When the agent
settles, the run falls into the on-time path untouched: nothing below the
runner learns it was late, and a task closed by the agent mid-window still
stands by the usual rule. The closing record carries `timed_out` as well,
because history's reader collapses a run to its latest record — a mark only
on an earlier line would be erased by the very act of reading, and
`herdr-docket history` shows the flag as `timed-out`. One window, recorded
once, two readers: the file sees the deadline happen, the CLI sees that the
run ended late.

**Bounded: twice the timeout again, then the fleet gives the agent up.**
Waiting indefinitely would be the same silence with the polarity flipped —
a hung agent would hold its agent slot and the task lock forever and no
notification would ever fire. One more window of the same length was the
natural number and the wrong one: TASK-145 settled 99 minutes past its
90-minute deadline, so one window would still have dropped the fleet's own
evidence by nine minutes. Twice the timeout catches every late delivery the
fleet has actually seen, with room for runs shaped like them, and bounds a
hung agent at three times its timeout. Past that the run ends the way a
failure always ends — task Failed, notification — and its workspace
closes first, because a task the fleet has closed must have no agent still
working on it. Uncommitted work in a never-settled worktree goes with its
workspace: `uncommitted` on the record, the reason on the task, the whole
sentence in the notification. Destroyed and said, never destroyed silently
— and never preserved by breaking the rule that the fleet closes what it
has given up on.

**Rejected:** ending the agent at the deadline (the ticket's other option)
— a late success becomes destroyed work, and the pipeline never gets the
chance it exists for. A fixed wall-clock grace — 90 minutes mean something
else to a 15-minute persona than to a 90-minute one. Calling `Do` a second
time to "resume" — it would start another agent and re-submit the prompt.
Leaving the workspace open with the agent stopped — herdr has no agent-stop
that spans the workspace modes the fleet provisions, and the run's
workspace teardown is already the one gesture the Host owns for "this run
is over".

**Consequences:** `timeout_minutes` is the deadline at which a run is
marked late, not the moment it dies — docs/agents.md says so. A late run
holds its agent slot and the task lock up to three times its timeout;
`agent pause` remains the manual brake. The uncommitted-work guard does not
run on the give-up teardown: there is no workspace left to keep or doubt,
and the give-up note is the whole account. The fake hosts grew `Settle`,
so every runner test can script what an agent does after its deadline.

**Status:** accepted
