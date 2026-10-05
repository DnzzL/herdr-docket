- **Cap how many agents run at once.** `max_runs` in `fleet.yaml` limits the
  runs in flight across the fleet, and per queue under it — past a few
  coding agents, one machine starves them all of memory and every run times
  out together.
- **`stop` and `pause --now` leave stale records alone.** A record no live
  run is behind could name a workspace id Herdr has since reused; it is
  marked `stale?` and never closed for you.
