- **The worktree registrations runs left behind before they retired
  themselves are swept.** `herdr-docket worktree sweep` walks every checkout
  the fleet's agents work in and retires the registrations of runs that ended
  cleanly, under the same rules a finished run's own close uses: clean trees
  pruned, a branch deleted only when a remote holds its exact tip, a dirty
  tree kept whole. It cross-checks the run history and herdr's open
  workspaces first — a run still in flight, a failed or blocked run kept as a
  resume point, a workspace somebody still has open, and any registration no
  run in history claims are all left alone, each with its reason in the
  report. `--dry-run` prints that report without touching git.
