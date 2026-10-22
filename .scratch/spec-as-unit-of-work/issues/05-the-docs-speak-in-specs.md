# 05: The docs speak in specs

**What to build:** a reader of the README, how-it-works and CONTEXT learns the fleet as it now is. The unit of work is a spec, the intake is a grill that ends with a spec and tickets committed under `.scratch/`, and the card carries a `Spec:` line plus the spec's `Done when` / `Verify by`. `.scratch` is the implementer's plan and triggers nothing: the card is the only ticket. CONTEXT also maps docket's words to the software-factory language (task → ticket, daemon → orchestrator, agent role → task, Herdr workspace → runner, queue backend → ticket tracker), with no rename in code. Docs state what is, not how it got there.

**Blocked by:** 02, 03, 04

**Status:** resolved

- [x] README's diagram, "How it works" and autonomy table describe the spec-sized pipeline and no factory loop
- [x] how-it-works describes the card `Spec:` convention and the grill intake
- [x] CONTEXT defines spec, ticket and card, and carries the mapping to the software-factory language
- [x] No doc describes one PR per task as the unit, or the factory loop

**Done when:** the three docs match the fleet after 02, 03 and 04.

**Verify by:** `grep -rn -- "--factory\|intake persona\|lookback" README.md docs CONTEXT.md` finds nothing; read the README top to "How much autonomy" against ADR 0021.
