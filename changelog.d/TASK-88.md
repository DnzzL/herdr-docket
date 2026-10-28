- **An idea filed with nobody named goes to the queue's triager.** On a queue
  that names a `triager:`, `herdr-docket task create "…"` files the card
  straight into `triage` in one write, so the triager grills it instead of the
  default agent building it unclear. A card with `-a`, or a queue with no
  triager, lands as before. `herdr-docket queues` lists the queue names for
  scripts.
- **`scripts/termux/idea` files an idea from the phone in one tap.** A
  Termux:Widget shortcut: pick the queue, dictate the sentence, and it runs
  `herdr-docket task create` on the box over ssh and toasts the new id.
  `--dry-run` prints the ssh command it would run.
- **`scripts/box-setup` sets up an always-on box.** A re-runnable wizard for
  Tailscale, the Herdr server, `gh` and agent CLI logins, the plugin, `init
  <repo>` per repo, one `intake` workspace per repo, and a `max_runs` sized
  from RAM. Each step checks before acting.
- The README gains a **Day to day** section: the phone loop and the laptop
  loop, with the widget's install steps.
