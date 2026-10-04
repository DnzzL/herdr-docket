- **The gate reads CODEOWNERS from the PR's base branch.** The merge gate
  used to read the file from the author agent's checkout on disk: a checkout
  behind the base protected what its stale copy said, and one on a branch
  without the file protected nothing. Now the gate fetches CODEOWNERS from
  the forge at the ref the PR targets (`.github/`, root, `docs/` — GitHub's
  own order); no file on the base still means no protected path, and a forge
  that cannot answer holds the task rather than merging unguarded.
