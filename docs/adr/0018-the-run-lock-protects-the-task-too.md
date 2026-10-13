# The run lock protects the task too

The failure this answers: a task unassigned and mid-run under one default
agent was picked up by another within one tick of a human changing
`default_agent:` in `fleet.yaml` — two workspaces, two agents, one task, both
runs live. The runner's lock was keyed per running unit (`run-agent-<name>`
in worktree mode, `run-root-<checkout>` in root mode), which keeps two runs
off the same *workspace* — and said nothing about the *task*. The old
assumption, "a task routes to exactly one agent, so holding that agent's slot
is holding the task", only holds while routing is stable, and routing is
re-read every tick from a file a human can edit mid-run.

**Two flocks per run, not a bigger one.** The unit a run mutates is what the
existing key protects — the agent's serial identity or the shared checkout;
that stays. But a run also claims one thing no other run may take whatever
the routing says, so `Run` now takes a second flock keyed by the task id
(`run-taskid-<id>.lock`, same state dir, same mechanics). A run without both
keys starts nothing; a run that cannot take both takes none. Two Runners —
the daemon's and a manual `herdr-docket run`'s, in two processes — contend on
the same files, so the claim holds across processes as before.

**Flock dies with the process, and that is the property the self-heal wants.**
A crashed run releases its task without a stale-lock file to notice or a
phase to trust: `In Progress` stays an open phase pick re-reads, and the
per-task lock — not the phase — is what refuses the second run while the
first is live. The daemon-local alternative (a set of running ids) is one
process deep, so a manual run and the daemon still race; the phase-guard
alternative needs the same in-flight knowledge and resurrects stale
`In Progress` false-negatives. Rejected both for that: the cross-process
flock is the smallest mechanism that answers both halves.

Reconsider when a fleet wants more than one run of a task at once — there is
no such feature, so there is no such key.
