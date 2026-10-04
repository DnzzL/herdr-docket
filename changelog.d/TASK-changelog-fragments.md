- **A pull request no longer edits the changelog.** Two open pull requests
  used to conflict on `CHANGELOG.md` every time, because both appended to the
  top of the same `## Unreleased` section — and here a conflict costs more
  than a rebase: the rebase changes the diff's patch-id, which voids the
  verifier's verdict, so merging one pull request sent every other one back
  for re-verification. A change worth announcing now writes one file under
  `changelog.d/`, named after its task, and distinct filenames never
  conflict. `herdr-docket changelog` previews what the next release will say;
  `herdr-docket changelog release vX.Y.Z` folds the files into a release
  section and deletes them. An older `## Unreleased` section is folded in by
  the next release, so nothing has to be moved by hand.
