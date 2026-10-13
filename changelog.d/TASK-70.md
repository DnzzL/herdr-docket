- **A held task settles when its pull request does.** A task parked in
  `Blocked` — or closed failed — used to stay there after you merged its PR,
  so your columns filled with work that had already shipped. Every few minutes
  the fleet now reads the PR of each held or failed task through the forge and
  settles the task by the PR's own state: merged closes it `Done`, a PR you
  closed unmerged closes it `Failed` with a note saying a human did, an open PR
  leaves the hold standing. The fleet never merges — it only reads what you
  already decided.
