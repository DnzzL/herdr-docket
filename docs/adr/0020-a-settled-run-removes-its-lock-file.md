# A settled run removes its lock file

The failure this answers (TASK-73): the state dir carried one `run-*.lock`
file per task the fleet had ever run — 76 and counting, none ever removed. A
lock file is a claim, not a record: ADR 0018 made the flock itself the
authority ("flock dies with the process"), so the file is nothing but the
thing the flock is taken on, and it outlived every run that ever took it. The
dir grows with every task, forever.

**Removal rides on release, not on a janitor.** The claim has exactly two
holders — `acquire` (which takes it) and `release`/`releaseLocked` (which
gives it back), so `drop` is the one place a lock file is retired: unlink the
file first, while the flock still excludes other runs, then unlock and close.
The file stops meaning "held" at the same moment the flock stops enforcing
it; a new opener may already be creating the next file before this descriptor
closes, and that is correct — the run has settled.

**The other half is in `claim`, and it is the subtle one.** Unlink-while-held
leaves one instant a naive reading would call a hole: a descriptor opened
just before the unlink, flocked just after the close, holds a claim on a file
nothing else can find — and a second run could be claiming the replacement
in parallel. So `claim` verifies, right after taking the flock, that the
file it locked is still the file at the path (dev + inode); a descriptor that
lost the race closes and claims again on the live file. Exclusion is decided
by flocks on one inode, never by two inodes standing for one task.

**The sweep is the one-time half.** Files from before removal existed — and
from crashes, which never reach `drop` — are taken once, at daemon start:
`SweepLocks` opens each `run-*.lock`, takes its flock non-blocking, and drops
the file only when the flock is takeable. A live claim — the daemon's own, or
a manual `herdr-docket run`'s in another process — passes the test and keeps
its file. From then on the steady state needs no sweep: a crash's leftover is
removed by the next claim/release pair on that key, and a key never claimed
again is a file the sweep already took.

**`drop` unlinks only its own file.** The sweep has a window of its own: it
opens each lock file and may be descheduled before it flocks; meanwhile the
run settles, the file is unlinked, and a new run claims the fresh file at
the same path. The sweep resumes holding a descriptor on a dead inode whose
flock now takes — and unlinking by path there would remove the next run's
live claim, handing the task to a third run while the second still holds
it. So `drop` unlinks only while its descriptor is still the file at the
path (the dev+inode check `claim` makes): a stale descriptor unlocks and
closes without touching another run's file.

**Alternatives rejected.** *Remove after unlock+close*: the gap where the
file exists but nobody holds it invites a second descriptor onto a file about
to be unlinked under a live claim — the exact window this must not open.
*Remove on acquire instead of release*: a crashed run's file would go on the
next acquire, but a settled run's file would still linger until some future
run reuses the key, and removal while another process is mid-`OpenFile` is
the same race with a longer window. *Keep the files, sweep by age*: keeps the
failure (the dir grows), guesses at liveness the flock already answers.

Reconsider when a lock file becomes something to read — a pid, a timestamp, a
holder's identity. Then it is a record after all, and removal needs a
retention rule instead of a release hook.
