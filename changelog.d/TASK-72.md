- **`task create --help` prints the usage instead of creating a task.** The
  flag was taken as the title: the queue's own CLI printed its help, named no
  id back, and the run still announced a creation — `created docket/ (in To
  Do)` for a task that never existed. `--help` and `-h` now print the usage,
  exit 0 and touch no queue, and a queue that returns no id is an error, never
  a creation.
