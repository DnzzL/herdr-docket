# changelog.d

One file per pull request. A change worth announcing writes **one file here**
instead of editing `CHANGELOG.md`, so two open pull requests never conflict
over the same section ([ADR 0016](../docs/adr/0016-a-pull-request-writes-a-changelog-file-not-a-changelog-section.md)).

Name it after the task your run carries — `TASK-62.md` — or after the change
if there is no task. The body is the bullet exactly as it should read in the
changelog: what this means for someone using the plugin, then the why in
their terms, not the refactor's.

```markdown
- **A run that times out keeps its delivery.** A late pull request used to be
  dropped on the floor; it now still reaches the pipeline.
```

Preview what the next release will say:

```sh
herdr-docket changelog
```

Cut it — writes the section into `CHANGELOG.md` and deletes the files it
consumed:

```sh
herdr-docket changelog release v1.2.3
```

This README is not a fragment; the command skips it by name.
