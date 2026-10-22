- **The factory loop is gone; `init --factory` now fails.** The intake, stall
  and lookback personas and their four schedules (intake, stall-sweep,
  lookback, merge-digest) no longer ship. `init --factory` exits non-zero and
  points at plain `init`, which writes the dev and reviewer roles. Entries an
  earlier install appended to `automations.yaml` stay until you delete them.
  A held pull request still raises the Herdr notification and the
  `merge-needed` label.
