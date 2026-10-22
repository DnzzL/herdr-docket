Status: ready-for-agent

# The unit of work is a spec

## Problem Statement

The owner drops an idea and wants it carried to a merged pull request without
babysitting. Today the fleet's unit of work is a single task: one run, one
worktree, one branch, one pull request per task. An idea worth five tasks
becomes five pull requests, verified and gated one by one, and the runner owns
a lifecycle per run whose leftovers have been the main source of bugs (a verify
run on the human's checkout, branches and registrations outliving the run,
sweeps retiring what earlier runs left).

The fleet also ships a factory loop — intake, pm, stall and lookback personas
plus four schedules behind `init --factory` — that is entirely disabled in the
owner's fleet, and an author role that mostly restates what the engineering
skills (`tdd`, `code-review`, `implement-spec`) and the repo's own
instructions already say. Two copies of each rule drift apart.

## Solution

An idea is grilled with the owner, written as a spec and a ticket graph with
`to-spec` and `to-tickets` in local markdown mode, committed on the queue's
base branch, and filed as one card that points at the spec. The author picks
up that card and runs `/implement-spec`: one integration branch, subagents per
ready ticket, a final code review, one pull request. A card that points at no
spec is worked as a single ticket with `/tdd`. The verifier then checks the
spec's and every ticket's `Verify by` against that one pull request, and the
gate merges as it does today.

Nothing in the runner, the queue adapters or the gate changes. The change is
in what ships with `init` (the two pipeline roles move there), what no longer
ships (the factory loop), what the two roles say, and the docs.

## User Stories

1. As the owner, I want to drop an idea and grill it once with an agent, so that the work starts from a shared understanding instead of a one-line task.
2. As the owner, I want the grill session to end with a spec and a ticket graph written to `.scratch/<slug>/`, so that the plan is versioned with the code it changes.
3. As the owner, I want to file one card per spec, so that my board shows ideas in flight, not a dozen fragments.
4. As the owner, I want the card body to carry a `Spec:` line pointing at the spec, so that the author finds the plan without a new field on the queue.
5. As the owner, I want the card to carry the spec's own `Done when` and `Verify by`, so that the verifier has a spec-level bar beside each ticket's.
6. As the owner, I want every ticket in the graph to carry `Done when` and `Verify by`, so that no part of the pull request goes unproven.
7. As the author, I want my role to tell me to run `/implement-spec` when the card points at a spec, so that I work the ticket graph instead of improvising an order.
8. As the author, I want my role to tell me to run `/tdd` when the card points at no spec, so that small cards still work without a spec ceremony.
9. As the author, I want to use subagents when my runtime has them and work the graph alone when it does not, so that the fleet stays runtime-agnostic.
10. As the author, I want my role to keep the delivery verb (`task done <id> --pr <url> --note`), so that the pipeline still hands my pull request to the verifier.
11. As the author, I want my role to keep "block with the one question that would settle it", so that I stop instead of guessing.
12. As the author, I want my role to keep "never merge your own work", so that the gate stays the only merger.
13. As the author, I want my role to defer coding method to the engineering skills and the repo's AGENTS.md, so that I follow one source of each rule.
14. As the author on a rework, I want the verifier's note as my next brief on the same integration branch, and to fix only what it names, so that a FAIL is fixed in place and the review does not reopen from scratch.
15. As the verifier, I want my role to tell me to read the spec and every ticket before the diff, so that I judge the pull request against the whole plan.
16. As the verifier, I want to check each ticket's `Verify by` and the spec's, so that one unproven ticket fails the pull request.
17. As the verifier, I want my verdict verb and its pinning to the patch-id unchanged, so that the gate reads my verdict as before.
18. As the verifier, I want my evidence ladder and my "the checkout is not yours" rule kept, so that what makes my verdict trustworthy is not lost.
19. As a new user, I want plain `herdr-docket init` to write `roles/dev.md` and `roles/reviewer.md`, so that the pipeline's two roles exist without installing a loop.
20. As a user with my own roles, I want `init` to never overwrite a role I already have, so that my edits survive a re-run.
21. As a user who scripted `init --factory`, I want a clear error saying the flag was removed, so that I do not silently get less than I asked for.
22. As a maintainer, I want the shipped roles pinned verbatim to `docs/examples.md`, so that the manual and the binary cannot drift.
23. As a maintainer, I want the factory package and its personas and schedules gone, so that no disabled code is left to maintain.
24. As a maintainer, I want holds to keep their herdr notification and `merge-needed` label, so that removing the daily merge digest loses only the digest.
25. As a reader of the README, I want it to say the unit of work is a spec and the intake is a grill, so that the front page matches the fleet.
26. As a reader of how-it-works and the factory doc, I want the pipeline described at spec size, so that the docs are state, not history.
27. As a reader of CONTEXT.md, I want the glossary to define spec, ticket and card as the fleet uses them, so that agents name things one way.
28. As a reader of CONTEXT.md, I want docket's terms mapped to the software-factory language (task → ticket, daemon → orchestrator, agent role → task, Herdr workspace → runner, queue backend → ticket tracker), so that Matt Pocock's skills and docket's docs read as one vocabulary without renaming the CLI.
29. As the owner, I want the first real spec after this one to go card → one pull request → PASS → gate with no human step, so that the factory is proven end to end.

## Implementation Decisions

- **Recorded in ADR 0021** (the unit of work is a spec, run by one implementer); it amends ADR 0013 and keeps its pipeline, verdict and gate.
- **No change to the queue seam.** `work.Task` gains no field; the four queue adapters are frozen. The spec pointer is a text convention in the card body: a line `Spec: .scratch/<slug>/spec.md`.
- **No mode in the runner.** Whether a card is a spec or a single ticket is decided by the author's role prompt, not by code.
- **`.scratch` is a plan, not a tracker.** The card is the only ticket that triggers a run; the ticket files are the implementer's plan and trigger nothing.
- **Spec storage** follows the local-markdown issue tracker configured in `docs/agents/issue-tracker.md`: `.scratch/<slug>/spec.md` and one file per ticket under `issues/`, blocking edges as text. The grill session commits them on the queue's base branch before filing the card.
- **The author runs in worktree mode**, as today; its worktree is the integration branch. Subagent worktrees belong to the agent runtime, not to the runner.
- **Roles move to `init`.** The `dev` and `reviewer` roles are embedded and written by plain `init`, beside the existing example role, skipping any role file that already exists and saying so.
- **The factory loop is removed**: the factory package, the `--factory` flag, the intake, stall, lookback, dev and reviewer personas it wrote, and its four schedules (intake, stall-sweep, lookback, merge-digest). `init --factory` exits non-zero with a message that the flag was removed and plain `init` writes the roles.
- **Author role** shrinks to: spec → `/implement-spec`, else `/tdd`; subagents if the runtime has them; deliver with `task done --pr --note`; on a rework, the verifier's note is the brief on the same branch and only what it names is fixed; block with one question; never merge.
- **The author reads `implement-spec`, it does not invoke it.** Pocock's `implement-spec` is user-invoked only (`disable-model-invocation`), and the owner keeps that flag machine-wide. The author role therefore says: read the skill's `SKILL.md` with the file tool, never the Skill tool, and follow it as the procedure; it names where the file usually lives (`~/.agents/skills/implement-spec/SKILL.md`). Probed 2026-10-22 on headless Claude Code: it reads and follows the file with no refusal. `tdd` and `code-review` are model-invocable and need no workaround.
- **Verifier role** keeps its content and gains one rule: read the spec and every ticket before the diff, and check every `Verify by`.
- **Docs**: README, how-it-works, factory and CONTEXT are rewritten to the spec-sized pipeline and grill intake; CONTEXT gains a mapping to the software-factory language, with no rename in code; docs/examples keeps the personas a fleet copies by hand and shows the two roles verbatim.
- **Owner's fleet config is outside this spec** (timeouts, model, removing disabled personas) and is done by hand after merge.

## Testing Decisions

- A good test drives a public entry point and checks what a user would see: files written, output printed, exit status. No test reaches into embedded variables or private helpers.
- **`init`**: writes both roles into a fresh fleet; keeps a role that already exists and reports the skip. Prior art: the two role tests of the factory install suite, moved to the fleet package.
- **Drift**: the shipped roles appear verbatim in `docs/examples.md`. Prior art: the factory drift test, kept for roles only.
- **CLI**: `init --factory` exits non-zero and names the removal. Prior art: the existing CLI tests on `task create` usage and errors.
- **Role content** has no unit test; it is proven by the end-to-end run in Further Notes.

## Out of Scope

- Any change to the runner, daemon, pipeline loop, lock files, gate or queue adapters.
- New fields on `work.Task` or parent/dependency support in any adapter.
- GitHub native auto-merge or branch protection.
- A planner agent, a docket-specific spec skill, or a fork of the engineering skills.
- An automated feedback loop (e.g. analytics-driven intake); may come later.
- The owner's `~/fleet` edits: author timeout, model choice, deleting disabled personas.
- Closing TASK-65, TASK-68 and TASK-69 as obsolete — done after this lands.

## Further Notes

**Done when:** this spec is merged and a real spec of at least two tickets on
herdr-docket goes card → one pull request → verifier PASS → gate with no human
step.

**Verify by:** `go test ./...` green; `herdr-docket init` into an empty dir
writes `roles/dev.md` and `roles/reviewer.md`; `herdr-docket init --factory`
exits non-zero naming the removal; then the end-to-end run above, read from
`herdr-docket history`.

This spec is the first one through the new flow and is run by hand, before the
daemon has worked any spec.
