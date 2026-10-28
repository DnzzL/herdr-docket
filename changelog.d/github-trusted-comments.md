- **A stranger's comment no longer reaches an agent on a GitHub queue.** Every
  comment on an issue used to become a note in the agent's prompt, so anyone
  could steer an agent on a public repository. Only the fleet's own login and
  the logins in `source.github.trusted` are read now; the others are counted.
