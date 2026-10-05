- **Pause the whole fleet, and see and stop its runs.** `herdr-docket pause`
  stops the daemon starting runs and holds a pipeline's next stage until
  `resume`, so the factory stops spending credits without touching each
  agent; `pause --now` also stops what is in flight. `herdr-docket runs`,
  `stop <task>` and a **runs** overlay pane show the runs in flight and stop
  one with a key.
