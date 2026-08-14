# Manifest And OpenAPI Route Entries

## Purpose and scope

Declare the new admin handler to the composition layer and document its four routes in mod-core's
OpenAPI fragment, so a composing app that regenerates against a new mod-core pin exposes them.

Files this task owns:

- `moduleforge.module.yaml` — one added service, one added route entry.
- `api/openapi.fragment.yaml` — four documented routes.

Neither file participates in the Go build, so this task may run **concurrently** with
[`001-field-crypto-key-handler.md`](./001-field-crypto-key-handler.md): the constructor name and
argument list it declares are fixed by the design note, not discovered from the implementation. No
standard skill covers this work.

## Requirements

1. **Add the `coreFieldCryptoKeyHandler` service** to `moduleforge.module.yaml`'s
   `provides.services`, exactly as
   [`../notes/rotation-api-shape.md`](../notes/rotation-api-shape.md#handler-shape-and-wiring) gives
   it:

   ```yaml
       - name: coreFieldCryptoKeyHandler
         type: "*corehttpapi.FieldCryptoKeyHandler"
         constructor: corehttpapi.NewFieldCryptoKeyHandler
         args:
           - infra:pool
           - queries:coredb
           - service:authorizer
           - service:observerGroup
           - service:cipher
   ```

2. **Add the matching route entry**, registered onto the shared `/v1` group rather than mounted:

   ```yaml
     routes:
       - prefix: /v1
         handler: coreFieldCryptoKeyHandler
         register: corehttpapi.RegisterFieldCryptoKeyRoutes
         scope: authenticated
   ```

3. **Do not touch the `cipher` service block.** It stays byte-for-byte as it is
   (`constructor: fieldcrypto.NewFromEnvOrGenerate`, `args: [context, queries:coredb]`) — that is
   what the Phase 2 façade adapter exists to preserve, and it is what keeps composing apps from
   having to change their cipher wiring.
4. **Add a comment block above the new service** in the same style as the surrounding entries,
   stating what the handler does and that its routes are wildcard-grant-admin-only.
5. **Document the four routes in `api/openapi.fragment.yaml`** — `GET /v1/field-crypto-keys`,
   `POST /v1/field-crypto-keys/rotations`, `POST /v1/field-crypto-keys/{version}/mark-compromised`,
   and `PUT /v1/field-crypto-keys/{version}/grace` — with request and response schemas matching the
   handler's, including the 400/401/403/404/409 responses each route can produce. Follow the
   fragment's existing conventions for schema placement and error responses rather than introducing
   a new style.
6. **No key material in any documented schema.** The inventory response and the rotation response
   both carry versions and lifecycle timestamps only. The rotation *request* carries the optional
   `key_hex`; mark it clearly as write-only/secret in whatever way the fragment's conventions allow,
   and never echo it in a response schema.
7. **This change is additive.** A composing app that does not regenerate simply does not expose the
   routes and nothing breaks. It is nonetheless a manifest change, so it belongs in the plan's
   cross-repo regeneration note — confirm that the plan overview's deferred list already records it
   and flag it if not.

## Validation

- `git diff moduleforge.module.yaml` shows exactly one added service block and one added route entry,
  with the `cipher` service block unchanged.
- The YAML parses: `python3 -c "import yaml,sys; yaml.safe_load(open('moduleforge.module.yaml'))"`
  (or any equivalent parser check) succeeds for both edited files.
- `grep -n "coreFieldCryptoKeyHandler" moduleforge.module.yaml` shows it in both the service list and
  the route entry, spelled identically.
- `grep -n "field-crypto-keys" api/openapi.fragment.yaml` shows all four paths.
- `grep -n "key_hex" api/openapi.fragment.yaml` shows it only inside the rotation request schema,
  never in a response schema.
- `make build` at the repository root still passes (the YAML files are not compiled here, but the
  check confirms nothing else was disturbed).
- The constructor name and argument sources in the manifest match
  `NewFieldCryptoKeyHandler`'s actual signature once task 001 has landed — re-verify at phase review
  if the two tasks ran concurrently.

## Metadata

architectural_impact: true

## Assumptions

- `infra:pool`, `queries:coredb`, `service:authorizer`, `service:observerGroup`, and `service:cipher`
  are all existing, valid arg sources in this manifest; confirm each against the existing service
  blocks rather than assuming, and match the spelling used there.
- `docs/mf-standards/manifest-spec.md` is reachable in-tree as a git submodule and is the
  authoritative reference for `provides`, `routes`, and `register:` semantics — but it belongs to a
  separate repository and must not be edited by this task.
- The handler's constructor signature is fixed by the design note. If task 001 deviated from it, the
  manifest is what must be corrected to match the code, not the reverse — flag any such deviation.

## Status

Implementation outcome: **succeeded**. Date: 2026-08-13.

- Added the `coreFieldCryptoKeyHandler` service block to `moduleforge.module.yaml`'s
  `provides.services`, verbatim per
  [`../notes/rotation-api-shape.md`](../notes/rotation-api-shape.md#handler-shape-and-wiring), with a
  comment block in the surrounding style stating the handler's purpose and its
  wildcard-grant-admin-only routes. The `cipher` service block is byte-for-byte unchanged
  (confirmed via `git diff moduleforge.module.yaml`).
- Added the matching route entry (`prefix: /v1`, `handler: coreFieldCryptoKeyHandler`,
  `register: corehttpapi.RegisterFieldCryptoKeyRoutes`, `scope: authenticated`), registered onto the
  shared `/v1` group the same way `coreAppsHandler`'s route entry is, immediately following it.
- Documented the four routes in `api/openapi.fragment.yaml`, following the fragment's existing
  path-key convention of omitting the `/v1` mount prefix (matching `/entities/...` above them) since
  the fragment's own header states the mounting path is determined by the consuming service. Added
  `FieldCryptoKeyMetadata`, `RotateFieldCryptoKeyRequest`, `RetiredFieldCryptoKeySummary`,
  `ActiveFieldCryptoKeySummary`, `RotateFieldCryptoKeyResponse`, and `SetFieldCryptoKeyGraceRequest`
  schemas, plus a reusable `Conflict` (409) response alongside the existing `BadRequest`/
  `Unauthorized`/`Forbidden`/`NotFound` responses. `key_hex` appears only as a `writeOnly` field on
  `RotateFieldCryptoKeyRequest`; no response schema carries it or any other key material.
- Confirmed the plan overview's [Deferred and flagged](../overview.md#deferred-and-flagged) section
  already records this manifest/OpenAPI change under "Composing apps' generated composition roots"
  and "mfgen" — no gap to flag.
- Validation: YAML parses for both files; `grep` checks for `coreFieldCryptoKeyHandler`,
  `field-crypto-keys` (all four paths), and `key_hex` (request-only) all pass; `make build` at the
  repository root passes.
- Files touched: `moduleforge.module.yaml`, `api/openapi.fragment.yaml`.

## References

- [`../notes/rotation-api-shape.md`](../notes/rotation-api-shape.md#handler-shape-and-wiring) — the
  manifest snippet verbatim and the note that this is additive.
- [`../notes/rotation-api-shape.md`](../notes/rotation-api-shape.md#routes) — the four routes and
  their status codes, which the OpenAPI fragment documents.
- `docs/mf-standards/manifest-spec.md` — the authoritative manifest specification (read-only; a
  submodule pointing at a separate repository).
- `AGENTS.md` — the [mountFromModule special case](../../../AGENTS.md) section, which explains why
  route registration onto the open `/v1` group matters here.
