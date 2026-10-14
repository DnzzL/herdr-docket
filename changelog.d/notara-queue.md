- **A Notara database can be the queue.** `kind: notara` works a database of
  a Notara server over its REST API: cards are picked, noted, assigned and
  closed through the same `task` commands, the page's to-dos are the
  acceptance criteria, and `herdr-docket auth notara --token ntr_…` stores the
  key and adds the fleet's statuses and agents as options of the database's
  selects. Its `statuses:` block is required — a database has no lifecycle the
  fleet can assume.
