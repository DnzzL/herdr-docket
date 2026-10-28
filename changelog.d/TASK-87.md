- **`herdr-docket status` says whether the daemon is alive:** each tick writes
  a heartbeat, and the command prints `daemon live · tick 3s ago`, `daemon down
  for 4 min` once no tick has run for three intervals, or `daemon never started`.
- **The pane reads from a phone.** `herdr-docket pane` opens on the daemon
  line, then "Your turn" — question cards (`?`), Blocked cards (`!`) and PRs
  verified PASS that wait on your merge (`⇡`) — then runs in flight and the
  last 5 settled runs, every line cut to the terminal width.
- **`herdr-docket questions` answers a grill round from a phone:** it lists the
  `question` cards, walks the newest round one question at a time — Enter takes
  the `(reco)`, a letter that option, other text a free answer — shows the
  line (`1.A 2.B 3. keep it`) and sends it as `task answer` on `Y`. Nothing it
  prints is wider than 45 columns.
