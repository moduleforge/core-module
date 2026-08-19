# mfgen composition wiring for the display registry

## Purpose and scope

Records how mod-core's own `moduleforge.module.yaml` must declare the display registry so an
mfgen-generated app constructs it, and — for the documentation deliverable — how a *downstream*
module (e.g. mod-users) is expected to register a renderer for its own entity type without any
module-to-module import and without hand-written wiring in the composing app. Findings come from
`docs/mf-standards/manifest-spec.md` (mod-core's submodule copy) and a read-only inspection of
`/Users/zane/playground/moduleforge/mfgen`.

## mod-core's own wiring

Three manifest edits, all in `moduleforge.module.yaml`:

1. A new `provides.services` entry `displayRegistry` (type `*display.Registry`) whose constructor
   builds the registry **and** registers mod-core's builtins, taking `queries:coredb`.
   The manifest spec explicitly blesses a side-effecting constructor: "A constructor may itself
   perform first-boot, DB-backed bootstrap work ... it is any Go expression, and the compiler emits
   a plain call to it" (`provides.services[]` section, using mod-core's own
   `fieldcrypto.NewFromEnvOrGenerate` as the shipped example).
2. A new `provides.services` entry for the display service, taking `service:displayRegistry` and
   `service:entityResolver` (both already provided by mod-core).
3. The existing `coreDeps` entry's `constructor` switches to the three-argument variant, adding the
   display service as a third arg.

Arg-source forms used are all already in use in this manifest (`queries:`, `service:`); no new
arg-source kind is required.

## Reachability — why a dangling registration service does NOT work

`mfgen/internal/resolver/reachability.go` prunes the construction graph: `ReachableNodes` keeps only
nodes transitively reachable from a root set of **handlers, middleware, and observers** (plus nodes
pinned by per-module `hooks:` args and the app's `startupHooks:` args). A `provides.services` entry
that nothing else consumes is **not** emitted into the generated `main.go` at all — so a
"registrar service" whose only purpose is its constructor's side effect would silently never run.

(The `_ = varName` unused-variable guard in `mfgen/internal/codegen/templates/main.go.tmpl` applies
to the **middleware** block only, not to services — it is not an escape hatch here.)

mod-core's own chain is safe: `coreDeps` (consumed by the `/v1` `mountFromModule` route) →
display service → `displayRegistry`, so every node is reachable from a route root.

## The downstream-module registration pattern (documentation deliverable)

A downstream module registering a renderer for its own type must attach its registration call to a
node that is reachable. Two viable mechanisms, both already in mfgen:

- **Fold the registration into a service the module already provides and that its own routes
  consume.** The module's existing handler/service constructor takes `service:displayRegistry` as an
  additional arg and calls `reg.Register(<itsTypeSlug>, display.FieldName, fn)` during construction.
  This is the lowest-ceremony option and needs no new manifest concept — the registry arrives as an
  ordinary `service:` arg, exactly as `service:authorizer` does today.
- **A module `hooks:` entry / the app's `startupHooks:`** — `reachability.go` pins `infra:`/`service:`
  args referenced by hook entries into the reachable set specifically so a hook can be the sole
  consumer of a service. This is the right shape when registration genuinely has no other home.

Either way the composing app writes **no** hand-rolled wiring: it selects modules, and mfgen
constructs one shared `*display.Registry` (a single provider name, so exactly one instance) and
threads it to every consumer. There is no module-to-module import — mod-users depends on mod-core's
`display` package (which it already does transitively via `core-api`), never the reverse.

**Verify before documenting.** `docs/mf-standards/manifest-spec.md` (the submodule copy mod-core
carries) documents `provides`, `requires`, `routes`, `observers`, and `middleware`, but does **not**
document the `hooks:` / `startupHooks:` fields that exist in `mfgen/internal/schema/{module,app}.go`.
Whichever mechanism the documentation task describes, it must be re-confirmed against the mfgen
source at the time of writing, and the doc must not present an undocumented mfgen field as though the
manifest spec sanctioned it — describe the service-arg form as the primary, spec-covered pattern and
mention the hook form as an mfgen capability with a pointer to the mfgen source.

## Out of scope

Modifying mod-users, mod-workflows, mfgen, or any app repo. The pattern is documented in mod-core
only; adopting it is each downstream module's own future work.
