# Admin Rotation API

## Purpose and scope

Phase summary for the operator-facing surface that actually triggers a rotation: a new
`FieldCryptoKeyHandler` in `api/httpapi/` with four admin-only routes, plus the manifest and OpenAPI
entries that expose them to composing applications. The full design is settled in
[`rotation-api-shape.md`](../notes/rotation-api-shape.md#b-admin-rotation-api).

## Goals

Without this phase the rotation machinery is inert: Phases 1–3 can store, read, and re-encrypt under
multiple key versions, but nothing can ever create a second version. The user's answer to the
lifecycle question was explicitly "new admin API/CLI endpoint", and the design pass established that
it must be **HTTP only** — mod-core has no `cmd/` directory and produces no binary, so a CLI would
mean introducing mod-core's first executable, its own database configuration, and a second
authorization story with no bearer token and no `opctx` actor.

The phase is deliberately separate from the read-path wiring: it is a different subsystem (a new
HTTP handler, its authorization rule, its observer dispatch, its manifest service block, and its
OpenAPI fragment), it depends only on Phase 2's cipher rather than on Phase 3, and it carries the
security-sensitive surface — wildcard-grant-admin-only authorization on every route including the
read-only inventory, and a hard guarantee that key material never crosses the HTTP boundary.

## Inputs

- Phase 1's `RetireActiveFieldCryptoKey`, `InsertActiveFieldCryptoKey`,
  `MarkFieldCryptoKeyCompromised`, `SetFieldCryptoKeyDecryptableUntil`, and
  `ListFieldCryptoKeyMetadata` queries.
- Phase 2's exported `Cipher.Reload`, called post-commit so the process that served the rotation
  converges immediately rather than waiting out the key-set TTL.
- The settled route table, request/response shapes, status mapping, authorization idiom
  (`az.Authorize(ctx, "manage", nil)`), and observer dispatch in
  [`rotation-api-shape.md`](../notes/rotation-api-shape.md#b-admin-rotation-api).
- `api/httpapi/apps.go`'s `AppsHandler` — the shape this handler follows field for field, including
  the `register:`-onto-the-shared-`/v1`-group route registration that avoids the duplicate-prefix
  panic.
- The settled rotation transaction ordering (retire, then insert, in one transaction) and its
  concurrency behavior from
  [`key-store-schema-design.md`](../notes/key-store-schema-design.md#rotation-transaction).

## Outputs

- `api/httpapi/field_crypto_keys.go`: `FieldCryptoKeyHandler`, `NewFieldCryptoKeyHandler`,
  `RegisterFieldCryptoKeyRoutes`, and the four routes — inventory `GET`, rotation `POST`,
  `mark-compromised` `POST`, and grace-window `PUT` — with the full status mapping including
  409 on a lost concurrent rotation and 400 when `compromised` and `grace_period_days` are combined.
- Observer dispatch for the rotation itself (`"rotate"` on resource `field_crypto_key`, target
  `nil`), carrying versions and lifecycle timestamps and never `key_bytes`.
- `moduleforge.module.yaml` gaining a `coreFieldCryptoKeyHandler` service and its route entry —
  additive, with the `cipher` service block untouched — and `api/openapi.fragment.yaml` documenting
  the four routes.
- An integration test proving the concurrency claim: two simultaneous rotations produce one 201 and
  one 409, and the table never holds two active keys.
- `cd api && make test`, `cd api && make lint`, and a whole-module `make build` passing.
