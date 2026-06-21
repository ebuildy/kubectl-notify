# AGENTS.md — kubectl-notify

> AI assistant instructions for the `kubectl-notify` project.  
> Maintained alongside code. Updated when conventions change.  
> Spec framework: [OpenSpec](https://openspec.pro/) — spec-driven development for AI coding assistants.

---

## Project Overview

`kubectl-notify` is a `kubectl` plugin written in Go that help Kubernetes ops to see events from Kubernetes or FluxCD.
Events can become desktop notifications (toasts), display in local web UI etc....

---

## Repository Layout

```
kubectl-sql/
├── AGENTS.md                    ← you are here
├── README.md
├── go.mod / go.sum
├── main.go                      ← CLI entrypoint (cobra)
│
├── cmd/
│   └── root.go                  ← root cobra command, flags, help
│
│
├── internal/
│   ├── port/                   ← Hexagonal interfaces definitions
│   ├── adapter/                ← Hexagonal ports implementations
│
├── pkg/
│   └── sqlschema/               ← public: well-known field aliases, type hints
│       └── schema.go
│
├── openspec/
│   ├── specs/                   ← long-lived behavioral specs (keep up to date)
│   │   ├── sql-grammar.md
│   │   ├── resource-resolution.md
│   │   ├── output-formats.md
│   │   └── error-enrichment.md
│   └── changes/                 ← active and archived feature changes
│       └── archive/
│
├── test/
│   ├── unit/
│   ├── integration/             ← uses envtest (controller-runtime)
│   └── fixtures/                ← sample kubeconfigs, YAML snapshots
```

---

## OpenSpec Workflow

This project uses **OpenSpec** for spec-driven development. Every non-trivial feature starts as a
change, not as a code edit.

### Quick reference

| Command | What it does |
|---|---|
| `/opsx:new <slug>` | Create a new change folder under `openspec/changes/<slug>/` |
| `/opsx:ff` | Fast-forward: generate `proposal.md`, `specs/`, `design.md`, `tasks.md` in one pass |
| `/opsx:apply` | Implement all tasks in `tasks.md` following the specs and design |
| `/opsx:archive` | Move completed change to `openspec/changes/archive/` and update long-lived specs |
| `/dev:update-readme-specs` | Regenerate the README `## Specs` table from `openspec/specs/*/spec.md` |

**After every `/opsx:archive`**, run `/dev:update-readme-specs` to keep the README `## Specs`
table in sync with `openspec/specs/`. A `PostToolUse` hook
(`.claude/hooks/after-opsx-archive.sh`) reminds the assistant to do this when it detects the
archive `mv` step — but it must still be run even if entries don't change (it is idempotent).

End propose a git commit message.

### When to create a change

- Adding or modifying SQL grammar (new clause, function, operator)
- New output format or renderer
- New Kubernetes resource type or API group support
- Changes to error enrichment logic
- Refactoring that touches more than two packages
- Any new CLI flag

### Change folder structure

```
openspec/changes/<slug>/
├── proposal.md     ← problem, scope, risks (readable in 2 minutes)
├── specs/          ← Given/When/Then scenarios — the behavioral contract
│   └── *.md
├── design.md       ← components, data flow, tradeoffs
└── tasks.md        ← ordered implementation checklist for /opsx:apply
```

### Long-lived specs (`openspec/specs/`)

These are the source of truth for stable behavior. After archiving a change, reconcile any
behavioral deltas back into the relevant spec file here.

---

## Coding Conventions

### Language and toolchain

- **Go 1.26+** — use the version pinned in `go.mod`
- **`golangci-lint`** — run before every commit: `make lint`
- **No global state** — all dependencies injected via constructors or context
- **Errors wrapped with context** — `fmt.Errorf("planner: %w", err)` at every boundary
- **`jq` for JSON** — parse and manipulate JSON on the command line with `jq`, never `python3 -c`

### Naming

- Packages: short, lowercase, no underscores (`parser`, `executor`, `k8s`)
- Exported types: full descriptive names (`SQLQuery`, `ExecutionPlan`, `RowSet`)
- Internal helpers: unexported, verb-first (`resolveField`, `buildFilter`)
- Test files: `<file>_test.go` in the same package (white-box) or `_test` package (black-box)
- Import of a port package must be suffixed by Port (ex: k8sPort "github.com/ebuildy/kubectl-sql/internal/port/datasources/k8s")

### Kubernetes client

- Use `k8s.io/client-go` dynamic client for all resource access (supports CRDs automatically)
- Never hardcode API group versions — discover them via the REST mapper
- Always respect `--context`, `--namespace`, and `--kubeconfig` flags
- Paginate LIST calls (default page size: 500) — never load the entire cluster into memory at once

---

## Build and Test

```bash
# Build
make build                  # produces ./bin/kubectl-sql

# Install locally as kubectl plugin
make install                # copies to ~/bin (must be on PATH)

# Lint
make lint                   # golangci-lint run ./...

# Unit tests
make test                   # go test ./... -race -count=1

# Integration tests
make e2e

# Coverage
make coverage               # opens HTML coverage report
```

Always call `make lint build test e2e` after edit go code.

### Testing rules

- Every new parser feature: unit tests in `test/unit/parser/`
- Every new SQL operator or function: at least one positive and one negative test
- Executor tests use `envtest` with fixture objects — no real cluster required
- No `t.Skip()` without a linked issue comment
- Test helper factories live in `test/fixtures/` — reuse them, never inline raw YAML in tests

---

## CLI Design

The plugin must conform to the `kubectl` plugin UX contract:

- Binary name: `kubectl-notify` (hyphen, not underscore)
- Installed on PATH → invoked as `kubectl notify`
- Respects all standard kubectl flags: `--kubeconfig`, `--context`, `--namespace`, `--token`
- Exit codes: `0` success, `2` k8s API error
- `--help` on every subcommand

---

## Git Policy

**The AI assistant MUST NOT run `git commit`, `git push`, or any command that writes to the repository history or remote.**

Only humans commit and push. The assistant's role is to write and edit files; the human decides when the work is ready to ship and runs git commands themselves.

This applies unconditionally — even if the user asks the assistant to commit or push in the same message. Write the files, then stop.

---

## Guardrails for the AI Assistant

1. **Read specs before coding.** If a relevant `openspec/specs/*.md` or change `specs/` file
   exists, read it first. Do not infer behavior from existing code alone.

2. **Do not modify `openspec/specs/` during a change.** Long-lived specs are updated only during
   `/opsx:archive` to reflect what actually shipped.

3. **Do not add dependencies without noting them in `design.md`.** Every new `go.mod` dependency
   must be justified in the active change's design doc.

4. **Do not write generated code by hand.** If a file has a `// Code generated` header, regenerate
   it via the appropriate `make generate` target instead.

5. **Preserve backward compatibility.** The SQL grammar is a public interface — removing or
   renaming clauses is a breaking change and requires a proposal.

6. **Security.** The plugin performs read operations (LIST, GET, WATCH) 

7. **Context resets between planning and coding.** After `/opsx:ff`, start a fresh session
   referencing the spec files — do not implement directly in the planning thread.

8. **One change = one responsibility.** Do not bundle grammar changes with output format changes.
   Keep changes narrow and reviewable.

---

## Work Flow

```
1. /opsx:new <slug>          # e.g. /opsx:new add-aggregate-functions
2. /opsx:ff                  # generate proposal + specs + design + tasks
3. Review output carefully    # adjust scope, add/remove scenarios
4. Start fresh session        # attach openspec/changes/<slug>/specs/ files
5. /opsx:apply               # implement tasks one by one
6. make lint && make test     # must pass clean
7. Open PR — include link to change folder in PR description
8. /opsx:archive             # after merge, reconcile openspec/specs/
9. /dev:update-readme-specs  # regenerate the README Specs table
```

---

*Last updated: 2026-06-04 — reconcile after each `/opsx:archive`.*