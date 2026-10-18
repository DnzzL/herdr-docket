- **A fleet gets its sandbox git credential from a wizard, not a spike.**
  `scripts/sandbox-git-token.sh` walks a human through minting the
  fine-grained token a sandboxed run pushes with — one repository, Contents
  and Pull requests read/write, 90-day expiry — proves it over https with a
  throwaway branch and a pull request it then closes, and stores the token
  nowhere: it is typed at the terminal and lives only in the environment.
