- **A verify run works in its own worktree at the PR's head, not a tab on
  your checkout.** The verifier used to run wherever its agent was
  configured — usually a tab on the project's primary checkout — with the
  isolation resting on a persona sentence that every reviewer rebuilt by
  hand (a throwaway worktree, a fetch of the PR head), spending its run's
  budget on setup and leaving refs, branches and registered worktrees behind
  in your repo. The fleet now pins the run itself: the PR's head commit is
  read from the forge, the run is provisioned in a fresh worktree cut at
  that commit whatever the agent's workspace mode says, and the worktree and
  its branch are removed when the run settles — verdict, failure,
  cancellation or timeout alike. A head the checkout does not have fails the
  run with the reason rather than fetching into your repo. The reviewer and
  plugin-reviewer personas lose the build-your-own-worktree paragraph, and
  the shipped example in the docs matches.
