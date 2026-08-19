# Wire Display Registry In Manifest

## Purpose and scope

Declare the display registry and display service in mod-core's `moduleforge.module.yaml` so an
mfgen-composed application constructs exactly one shared `display.Registry` — with mod-core's
builtins already registered — and threads it into the core httpapi router. Without this, the
endpoint added in task 002 exists but always answers `display_name: null`, because nothing in the
composition path ever builds a registry.

Scope is `moduleforge.module.yaml` plus the `AGENTS.md` rows that describe the manifest and the
affected packages. No Go source changes — tasks 001 and 002 already provide every constructor this
manifest references. No standard skill covers this; follow the procedure below.

## Requirements

### 1. Two new `provides.services` entries

Following the existing entries' formatting and comment style in `moduleforge.module.yaml`:

- **`displayRegistry`** — type `*display.Registry`, constructor `coreservice.NewDisplayRegistry`
  (the task-001 function that builds the registry *and* registers mod-core's builtins), args:
  `queries:coredb`. Comment it as the single shared registry every module's renderer registration
  targets, and note that the constructor performs registration as a side effect — the manifest spec's
  `provides.services[]` section explicitly sanctions a side-effecting constructor and cites
  mod-core's own `fieldcrypto.NewFromEnvOrGenerate` as the shipped precedent.
- **The display service** — type `*coreservice.DisplayService` (match whatever task 001 actually
  exported; use the concrete type, as the other entries do), constructor
  `coreservice.NewDisplayService`, args `service:displayRegistry` then `service:entityResolver`, in
  the order task 001's constructor declares. `entityResolver` is already a provided service in this
  manifest — do not add a second one.

Use the `coreservice` / `corehttpapi` import aliases the existing entries already use; introduce no
new alias.

### 2. `coreDeps` switches to the three-argument constructor

Change the existing `coreDeps` entry's `constructor` to `corehttpapi.NewDepsWithDisplay` and append
the display service as the third arg, after `service:coreServices` and `infra:logger`. Arg order must
match that function's parameter order exactly. Update the entry's comment to say what the third
dependency is and that a composing app which omits it (via the still-supported two-argument
`NewDeps`) simply gets the graceful `display_name: null` behaviour.

Leave every other manifest section — `routes`, `observers`, `requires`, `migrations`, `go`, `gui` —
untouched. In particular, add **no** new `provides.routes` entry: the display-name route is served by
the existing `/v1` `mountFromModule: corehttpapi.NewRouter` entry.

### 3. Verify the construction graph against mfgen (read-only)

mfgen prunes unreachable nodes, so a manifest entry that nothing consumes is silently never
constructed. Confirm, by reading `/Users/zane/playground/moduleforge/mfgen`
(`internal/resolver/reachability.go`, `internal/resolver/graph.go`,
`internal/codegen/templates/main.go.tmpl`) — **strictly read-only; make no edit anywhere under that
path** — that:

- `displayRegistry` and the display service are both transitively reachable from a root node
  (they are, via `coreDeps` ← the `/v1` route), so both are emitted into the generated `main.go`.
- The topological order is satisfiable: `displayRegistry` before the display service before
  `coreDeps`.
- Neither constructor `ReturnsError`, so no `returnsError: true` flag is needed on either entry
  (confirm against the actual task-001 signatures; add the flag if either does return an error).

Record the outcome in the task document's own notes. If any of these checks fails, halt and report
rather than guessing at a manifest shape that will not generate.

### 4. `AGENTS.md` updates

- The `display/` package row: note that a composing app obtains the shared registry via
  `coreservice.NewDisplayRegistry` and that mod-core's manifest provides it as `displayRegistry`.
- The `service/` row: mention the display service alongside the existing four servicers.
- The `httpapi/` row: mention `GET /v1/entities/{uuid}/display-name`, its authentication-only rule,
  and that it answers `200` with `display_name: null` rather than an error when no renderer is
  registered for the entity's type.
- Keep each addition to the existing terse row style; do not restructure the tables.

## Validation

- `moduleforge.module.yaml` parses as valid YAML (`python3 -c "import yaml,sys;yaml.safe_load(open('moduleforge.module.yaml'))"` or equivalent).
- Every `constructor:` and `args:` entry added names a symbol that actually exists at the signature
  used — cross-check each against `api/service/display.go` and `api/httpapi/router.go` by grep.
- `service:displayRegistry` resolves to exactly one `provides.services[].name` in this manifest, and
  `service:entityResolver` still resolves to the pre-existing entry (no duplicate names).
- `coreDeps`'s arg list length and order match `corehttpapi.NewDepsWithDisplay`'s parameters exactly.
- `git diff --stat` names only `moduleforge.module.yaml` and `AGENTS.md`; nothing under
  `/Users/zane/playground/moduleforge/mfgen` and nothing under `docs/mf-standards/` is modified.
- `cd api && make test` and `cd api && make lint` still pass (no Go change expected, run as a
  regression guard).

## Metadata

architectural_impact: true

## References

- [mfgen composition wiring](../notes/mfgen-composition-wiring.md) — the reachability finding, the
  arg-source forms, and why a dangling registrar service does not work.
- `moduleforge.module.yaml` — the file this task edits; `cipher` and `coreAppsHandler` are the
  closest-shaped existing entries.
- `docs/mf-standards/manifest-spec.md` §2 `provides.services[]` and §4 arg-source vocabulary
  (reference only — the submodule is never edited).
- `/Users/zane/playground/moduleforge/mfgen/internal/resolver/reachability.go` (read-only).
- Tasks `001-add-display-service.md` and `002-add-display-name-endpoint.md` — the exported symbols
  this manifest names.

## Checkpoint hints

- After the two `provides.services` entries are added.
- After the `coreDeps` constructor switch.
- After the mfgen read-only reachability verification.
- After the `AGENTS.md` row updates.
