- **Every `task verdict` example shows the `--note` flag.** The docs and the
  shipped reviewer role taught a bare verdict while the prompt required the
  note — an agent following them literally left its evidence in the task
  notes instead of on the PR comment. All examples now carry
  `--note "<evidence>"`, and a test keeps them that way.
