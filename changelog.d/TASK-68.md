- **A finished worktree run no longer leaves its worktree registered in
  your repo.** Every worktree-mode run used to end with its workspace closed
  but its git worktree and its `fleet/…` branch still sitting in the primary
  checkout — `git worktree list` grew a pile nobody pruned. When the fleet
  closes a run's workspace it now retires the worktree with it: the
  registration and checkout go, and the branch is deleted only when the
  agent's push put it on the remote unchanged. An unpushed branch, a dirty
  tree, or a workspace that would not close all stay untouched — nothing the
  run still holds alone is ever deleted (ADR 0018).
