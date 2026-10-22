Status: ready-for-agent

# Every ADR has its own number, and the author role speaks the glossary

## Problem Statement

Two ADRs carry the number 0020: the lock-file one and the Notara one. They
were written by two pull requests in flight at the same time, each taking the
next free number on its own branch. A reader who follows "ADR-0020" from a
code comment or a doc cannot tell which decision is meant, and nothing stops
the next two parallel pull requests from doing it again — more likely now
that one spec runs its tickets in parallel.

Separately, the `dev` role shipped by `init` says a card with no `Spec:` line
"is one ticket". The glossary says otherwise: a ticket lives only in
`.scratch` and never in the queue; a card with no spec is still a card,
worked as one ticket's worth of work. The role an author reads first uses a
word the glossary forbids for it.

## Solution

The Notara ADR, the later of the two, takes the next free number (0022), and
every reference to it follows. A test fails whenever two ADRs share a number,
so the collision cannot come back unnoticed. The `dev` role's no-spec line is
reworded to the glossary's terms, in the shipped role and its copy in the
manual.

## User Stories

1. As a reader of a code comment, I want "ADR-00NN" to name exactly one decision, so that I open the right record.
2. As a reader of the queues manual, I want its link to the Notara decision to land on that decision, so that the manual and the record agree.
3. As a maintainer of the lock-file code, I want the lock-file ADR to keep its number, so that the history and task notes that cite it stay right.
4. As a contributor opening an ADR in parallel with someone else, I want the test suite to fail when our numbers collide, so that one of us renumbers before merge.
5. As a contributor, I want that failure to name the duplicated number and both files, so that I know what to rename without searching.
6. As an author agent, I want my role to say a card with no `Spec:` line is worked as one ticket's worth of work, so that I use the glossary's words from my first read.
7. As a maintainer, I want the role in the manual to stay verbatim with the shipped one, so that the drift test keeps both honest.
8. As the owner, I want this spec to go card → one pull request → verifier PASS → gate with no human step, so that the spec-sized pipeline is proven end to end.

## Implementation Decisions

- **Which ADR moves**: the Notara ADR, added after the lock-file ADR, becomes 0022 (0021 is taken). Its file name and its own title line change; its content does not.
- **References that follow it**: the Go comments in the queue vocabulary and in the Notara and Backlog.md adapters that cite it, and the queues manual's link. These are comment and doc edits only; the frozen adapters gain no behaviour.
- **References that stay**: everything that cites the lock-file ADR as 0020, including task notes in the backlog.
- **The uniqueness test** lives with the root package's tests, reads the ADR directory, and fails listing each number used by more than one file, with the file names.
- **The role line** changes in the shipped `dev` role and its verbatim copy in the examples manual, nothing else in the role.

## Testing Decisions

- A good test here checks what a contributor would see: the suite going red with the duplicate named. It reads the ADR directory as it is on disk; it does not hard-code today's file list.
- **New**: ADR numbers are unique — red on the main branch before the rename, green after.
- **Existing**: the role drift test (shipped role == examples manual) covers the role change.
- Comment and link edits are checked by search, not by test.

## Out of Scope

- Renumbering any other ADR, or a tool that hands out ADR numbers.
- Any change to ADR content, to adapter behaviour, or to the rest of the `dev` role.
- Updating the owner's fleet copy of the role (`init` never overwrites it; done by hand).

## Further Notes

**Done when:** no two ADRs share a number, every reference to the Notara ADR
names 0022, the `dev` role uses the glossary's terms for a no-spec card, and
this spec merged through the pipeline with no human step.

**Verify by:** `go test ./...` green, including the new uniqueness test (red
on main before the rename); `ls docs/adr | cut -c1-4 | sort | uniq -d` prints
nothing; `grep -rn "ADR-0020\|0020-a-notara" internal docs` finds no Notara
reference; `grep -n "card is one ticket" internal/fleet/roles/dev.md
docs/examples.md` finds nothing; `herdr-docket history` shows the run
verified PASS and merged.
