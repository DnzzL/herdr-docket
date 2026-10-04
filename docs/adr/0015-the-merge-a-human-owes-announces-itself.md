# The merge a human owes announces itself, on the popup and on the PR

The failure this answers: the fleet stopped on a pull request only a human
can move — a gate hold (the `critical` label, a CODEOWNERS path, `merge:
never`, a refused merge), a verifier's second FAIL, a run that failed with
its PR still open — and the human heard about it the same way they hear
about every log line: a silent popup naming the *task*, if they were
looking. GitHub cannot carry the message either: the fleet opens PRs under
the human's own account, and GitHub never notifies you of your own actions.
The PR itself is where a merge-needed mark can still be found.

**One popup, titled by the PR.** Each of those stops raises exactly one
popup, `merge needed — <repo>#<number>`, the reason as its body, the
`request` sound — the title a human can act on without opening the board,
and a sound that distinguishes it from the rest. The old `fleet: held`
popup is that stop now, not a second one; a failed run with no PR out keeps
`fleet: run failed` as it always was, because there is nothing to hand over.

**The sound joins the port; the invocation stays behind it.** `host.Notify`
gains a third argument, a sound from the host's own vocabulary
(`SoundNone`, `SoundRequest`), and the runner names which one a stop
deserves — this amends ADR 0010's claim that the runner does not know the
sound. Which sound says "act" is policy, and the core owns what a human
reads or hears; what stays behind the port is the CLI invocation, the flag
and the position. Rejected: a second `Notify`-shaped method per tone (every
future tone grows the port), and the host inferring a tone from title words
(a convention no test can pin).

**The label rides the forge.** `gate.Forge` grows `AddLabel` and
`RemoveLabel`, because creating a label in a repo, applying it, and reading
a PR's labels before taking one off is gh's own multi-call choreography and
belongs in the adapter. A hold adds `merge-needed`; the gate removes it as
it merges, so a merged PR never reads as owed — the label marks a debt, and
the merge pays it. Both asks are best-effort like every notification (ADR
0010): the note on the task and the Blocked column are the report, the
popup is the voice and the label the durable trace.

**No state check before announcing.** Rejected: asking the forge whether
the PR is still open first. A failed read would silence exactly the
announcement this exists for, and the race it guards against — the PR
closing in the seconds before the stop — leaves at worst a stale marker a
human clears, which is the cheaper error against an unhearable one.

**Status:** accepted
