# The prompt is three layers, widest first; a brief nobody named is optional

A run's prompt is assembled from a small number of layers, and each layer
answers to the same question: who owns this fact? The rule that settles every
case is whether a human named the layer. A layer somebody explicitly named must
be present or the agent grounds; a layer nobody named is optional and its
absence changes nothing.

The layers are widest first — the fleet brief, then the agent's role, then the
persona. Each may be absent, and an absent layer changes nothing else.

**`FLEET.md` is optional because nobody named it.** Missing, empty, or entirely
commented, the prompt is byte-for-byte what it was before the file existed.
`init` scaffolds a wholly-commented `FLEET.md`, and because an all-comment file
counts as no brief, the untouched scaffold never reaches an agent — the brief
starts speaking the moment a human writes one uncommented line into it.

**A `role:` is a reference, so it must resolve.** `role: dev` in an agent's
frontmatter names `roles/dev.md`; a role naming a file that does not exist
grounds the agent rather than being ignored, because a run assembled without a
third of its instructions looks exactly like one that had them. The agent does
not load and `agent list` says why — not `Unavailable`, which means "fine, just
busy".

**There is no inheritance between roles.** A role file is plain markdown anyone
can write; a mechanism for two files is not a mechanism worth having.

**Status:** accepted

## Considered options

- **Make `FLEET.md` required.** Rejected: it would ground every existing fleet
  that has no brief, for a file most never had. Optional-but-preferred is the
  whole point — the scaffold is discoverable, not mandatory.
- **Ignore a `role:` that names a missing file.** Rejected: a silent third of
  the prompt, gone. The asymmetry with `FLEET.md` is exactly the rule above:
  nobody named the brief; somebody named the role.
- **Role inheritance (`role: dev extends base`).** Rejected: two files are not
  enough to justify a mechanism, and every layer it added would have to be
  tested and explained.
- **A goals cascade, company to agent, as Paperclip does.** Rejected: the brief
  is one file a human writes once, not a tree to maintain. The fleet keeps no
  goals model — see the plugin's scope.

## Consequences

- `prompt.Assemble` keeps a pure core (`assemble`) with the one file read in a
  thin wrapper, so the contract the whole system depends on is pinned by a test.
- The prompt states the queue's own status words for the task's project, and
  the same rule decides them: the fleet names the words it writes, and a board's
  human columns are not the fleet's to teach — so a backend with no statuses is
  simply told of none.
- A role is the seam for a shared method; a brief is the seam for shared
  context. Neither is the seam for per-task instructions, which belong on the
  task, in the queue.
