- **An agent can run inside a bubblewrap sandbox.** `sandbox:` in AGENT.md
  names a ZDOTDIR chain; the run's pane starts under it, so the shell
  resolves the agent's plain name to an agent-sandbox.nix wrapper while
  `herdr agent start` and its detection stay untouched. Sandboxed worktree
  runs are cut by the fleet and opened as a workspace (herdr's
  `worktree create` carries no environment); a sandboxed root run opens its
  own workspace instead of borrowing a tab. Herdr drives an agent through
  bubblewrap as it does on the host — proven end to end on a real dishnow
  task, with the verifier still running unsandboxed.
