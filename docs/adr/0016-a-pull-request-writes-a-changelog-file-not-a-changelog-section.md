# A pull request writes a changelog file, not a changelog section

Every pull request appended its entry to the top of `## Unreleased` in
`CHANGELOG.md`. Two open pull requests therefore inserted at the same anchor,
and git's three-way merge calls competing inserts at one anchor a conflict —
every time, with no exception, however unrelated the two changes were.

In this repo that cost more than a conflict costs elsewhere. Resolving it
means rebasing; rebasing changes the diff's `git patch-id`; and a changed
patch-id voids the verifier's recorded verdict, by design
([ADR 0013](0013-the-factory-is-an-author-a-verifier-and-a-gate-in-code.md),
`internal/gate`). So merging one pull request dirtied every other open one,
and each needed a fresh verification run to come back. Landing *n* changes
cost *n²* verifications. On 2026-10-04 six pull requests were open and all
six conflicted on `CHANGELOG.md` and on nothing else; merging three of them
took two rounds of re-verification for a file no reviewer reads.

**One file per pull request, named by whoever writes it.** A change that is
worth announcing writes `changelog.d/<name>.md` holding the bullet it would
have added. A fleet run names it after its task — `changelog.d/TASK-62.md` —
because that name is already unique and already means something. Distinct
filenames never conflict, so the changelog stops serialising the fleet.

**The section exists only at release.** `herdr-docket changelog` previews what
the next release will say; `herdr-docket changelog release v1.2.3` folds the
fragments into a `## v1.2.3 — <date>` section at the top of `CHANGELOG.md`
and deletes the fragments it consumed. One person runs it once per release.

**`CHANGELOG.md` stays the release workflow's source.**
`.github/workflows/release.yml` greps `## <tag> ` out of it for the GitHub
release body and is unchanged by this. The command emits exactly that shape
and refuses a version it could not find again, so the two cannot drift.

**Migration costs nobody a rewrite.** The `## Unreleased` section an older
`CHANGELOG.md` still carries is not stranded: the next release folds it in
beneath the fragments, then drops the heading. Nothing has to be moved by
hand, and a half-migrated repo works.

**Rejected:** excluding `CHANGELOG.md` from the verdict's patch-id — five
lines in the gate, but it only severs the verdict half. The pull requests
still go dirty and still need a human or a rebase, and it teaches the gate to
ignore a file, which is a rule to remember rather than a problem removed.
Generating the changelog from commit subjects at release — the subjects say
what was refactored, and this file is written for someone using the plugin
(`CONTRIBUTING.md`); that is the same reason `release.yml` prefers a written
section over `--generate-notes`. A conflict-tolerant merge driver
(`union`) — it resolves the text and still rewrites the diff, so the verdict
dies anyway, and it silently interleaves entries nobody reviewed.

**The same pathology, not fixed here:** ADR numbers are also a global counter
allocated when a change is authored rather than when it merges, which is why
three open pull requests each claimed `0014` on 2026-10-04 and git merged
them without a word, the filenames being distinct. Noted, not addressed.
