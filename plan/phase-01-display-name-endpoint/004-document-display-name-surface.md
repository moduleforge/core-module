# Document Display Name Surface

## Purpose and scope

Document the new display-name HTTP surface for its two audiences: API consumers (the OpenAPI
fragment) and the author of the future composing application and of a future downstream renderer
registration (a mod-core-owned architecture doc). Also draft — without committing — the
corresponding amendment to the `docs-mf-standards` repository's `architecture/entity-typing.md`.

Scope: `api/openapi.fragment.yaml`, a new document under `docs/architecture/`, `README.md`, and
`AGENTS.md`. **Nothing under `docs/mf-standards/` may be edited, staged, or committed** — it is a git
submodule of a separate repository. No standard skill covers this; follow the procedure below.

## Requirements

### 1. OpenAPI fragment

Add a `/entities/{uuid}/display-name` path with a single `get` operation to
`api/openapi.fragment.yaml`, matching the fragment's existing style (tags, `operationId`, `summary`,
`description`, parameters, response schemas, reuse of existing `components` error schemas rather than
new inline copies). It must document:

- `operationId: getEntityDisplayName`, tagged with the same `Entities` tag as the sibling entity
  routes.
- The `uuid` path parameter (format `uuid`).
- `200` response schema: an object with `uuid` (string, format `uuid`) and `display_name`
  (string, **nullable**), both always present. Include the two examples — a rendered name and a
  `null`.
- A description stating plainly that `display_name` is `null` — with a `200`, never an error status —
  when no renderer is registered for that entity's type, when the deployment wires no registry, or
  when the UUID names no entity; and that a client should fall back to rendering the raw UUID.
- `400` (`invalid_input`, malformed UUID) and `401` (`unauthenticated`) responses, referencing the
  same error components the fragment's other operations use.
- An explicit note that this operation requires **authentication only** — it does not require read
  access to the referenced entity — because its purpose is resolving an entity UUID a caller already
  holds as a reference in another module's data.

Do not add `403` or `404`: existence is deliberately not disclosed.

### 2. New mod-core-owned architecture doc

Create `docs/architecture/display-name-http.md` (`docs/architecture/` exists and currently holds only
a `CLAUDE.md`). It opens with `## Purpose and scope` per the project documentation conventions and
covers, in prose:

- **The endpoint contract** — path, method, both response shapes, status codes, and the
  authentication-only authorization rule with the rationale from
  [the design note](../notes/display-http-surface-design.md#authorization-decision). State explicitly
  that "no renderer registered" is an expected steady state (an entity type owned by a module absent
  from this deployment), which is why it is a `200` and not an error — and that the top-level
  `error.code` set is closed, so no new code could have been minted for it.
- **How the registry is wired in a composed app** — mfgen constructs one shared `*display.Registry`
  from mod-core's `displayRegistry` manifest entry (constructor `coreservice.NewDisplayRegistry`,
  which registers mod-core's `natural_person` / `corporation` / `service_account` builtins), threads
  it into the display service and thence into `corehttpapi.NewDepsWithDisplay`, and mounts the route
  with the rest of `/v1/entities/*`. The composing app writes no hand-rolled wiring; it selects
  modules.
- **How a downstream module adds a renderer for its own entity type** — the pattern, stated
  generically with mod-users as the illustrative (not-yet-built) example: the module ships its own
  `RegisterBuiltins`-equivalent following `api/service/display_builtins.go`, keyed on the
  `fundamental_type_slug` its own migration seeded, and reaches the shared registry as an ordinary
  `service:displayRegistry` arg on a constructor it already provides. No module-to-module import and
  no mod-core edit is required — this is the whole point of the pattern.
- **The reachability caveat** — mfgen prunes services nothing consumes, so the registration call must
  ride a node that is reachable from a handler, middleware, observer, or a `hooks:`/`startupHooks:`
  entry; a `provides.services` entry declared purely for its constructor's side effect is silently
  never constructed. Present the service-arg form as the primary, manifest-spec-covered pattern; if
  the doc mentions mfgen's `hooks:` / `startupHooks:` fields, re-confirm them against the mfgen
  source first and mark them as an mfgen capability not currently described in
  `docs/mf-standards/manifest-spec.md`, rather than presenting them as spec-sanctioned.
- **What is deliberately not here** — mod-users' own renderer, mod-workflows' GUI client call, and
  the composing app itself are all separate future work; this document describes the contract they
  will build against.

Link the design note's substance into the doc as prose — do not link to `plan/` paths, which are torn
down when the plan completes.

### 3. `README.md` and `AGENTS.md`

- `README.md`: add a link to the new doc so it is reachable from the project entry point, alongside
  the existing `AGENTS.md` / `manifest-spec.md` pointers.
- `AGENTS.md`: link the new doc from wherever the display/httpapi material sits, without duplicating
  its content. If task 003 already updated the `display/`, `service/`, and `httpapi/` rows, extend
  rather than restate.

### 4. Drafted, uncommitted cross-repo amendment

Draft a short addition to `docs-mf-standards`' `architecture/entity-typing.md` "Display-rendering
pattern" section noting that the pattern is now reachable over HTTP via mod-core's
`GET /v1/entities/{uuid}/display-name`, that the graceful not-registered fallback surfaces as
`display_name: null` with a `200`, and pointing at mod-core's own
`docs/architecture/display-name-http.md` for the wiring detail.

- Do **not** apply it. Do not edit, stage, or commit any file under `docs/mf-standards/`.
- Return the verbatim proposed text in the task's structured report.
- Record a followup (type `documentation`) describing the pending cross-repo amendment, in the same
  shape as the existing precedent followup `qxLX` recorded by the `require-authenticated` plan.

## Validation

- `api/openapi.fragment.yaml` parses as valid YAML and the new path's `$ref`s all resolve within the
  fragment (no dangling component reference).
- The `200` schema marks `display_name` nullable in a way consistent with the fragment's declared
  OpenAPI version (3.0.3 — use `nullable: true`, not a 3.1-style type union).
- `docs/architecture/display-name-http.md` exists, opens with `## Purpose and scope`, and is linked
  from `README.md`; the link resolves as a repo-relative path.
- `grep -rn "display-name" README.md AGENTS.md docs/architecture/display-name-http.md api/openapi.fragment.yaml`
  shows the endpoint documented consistently in all four, with one path spelling.
- `git status --porcelain docs/mf-standards` reports no modification, and `git diff --stat` names
  only `api/openapi.fragment.yaml`, `docs/architecture/display-name-http.md`, `README.md`, and
  `AGENTS.md`.
- The documented request/response shapes match the implementation exactly — cross-check the field
  names and status codes against `api/httpapi/display.go` and its tests rather than against this
  task document.
- `cd api && make test` and `cd api && make lint` still pass.

## Metadata

architectural_impact: true

## References

- [Display HTTP surface design](../notes/display-http-surface-design.md).
- [mfgen composition wiring](../notes/mfgen-composition-wiring.md).
- [Documentation ownership constraint](../notes/docs-ownership-constraint.md) — the submodule rule and
  the `qxLX` precedent.
- `api/openapi.fragment.yaml` — existing style for entity operations.
- `docs/mf-standards/architecture/entity-typing.md` "Display-rendering pattern" (read-only source for
  the drafted amendment).
- `plan/plan-summary-require-authenticated.md` — the `qxLX` cross-repo-amendment precedent.

## Checkpoint hints

- After the OpenAPI path entry.
- After the new architecture doc.
- After the `README.md` / `AGENTS.md` links.
- After the drafted amendment and its followup are recorded.
