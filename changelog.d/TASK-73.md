- **The state dir no longer grows a lock file per task, forever.** Each run's
  `run-*.lock` claim is removed the moment the run settles, so the plugin's
  state directory stops accumulating one file for every task ever run. The
  files older runs left behind are swept once when the daemon next starts,
  and a run that is still live keeps its lock and its exclusion exactly as
  before.
