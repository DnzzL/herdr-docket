- **A spec's ticket files are resolved by the pull request that delivers it.**
  The `dev` role now says to set every ticket file under the spec's `issues/`
  to `Status: resolved` before delivering, and the `reviewer` role FAILs a spec
  pull request whose head still has a ticket reading `ready-for-agent` — a
  merged spec no longer leaves the board claiming its slices are open.
