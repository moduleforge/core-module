# Field Crypto Key Admin Handler

## Purpose and scope

Build the operator-facing surface that actually triggers a rotation: a new `FieldCryptoKeyHandler` in
`api/httpapi/` with four admin-only routes. Without it the rotation machinery built by Phases 1–3 is
inert — nothing can ever create a second key version.

Files this task owns:

- `api/httpapi/field_crypto_keys.go` — new.
- `api/httpapi/field_crypto_keys_test.go` — new.

The manifest and OpenAPI entries are
[`002-manifest-and-openapi-routes.md`](./002-manifest-and-openapi-routes.md); the concurrency
integration test is [`003-rotation-endpoint-integration-test.md`](./003-rotation-endpoint-integration-test.md).
No standard skill covers this work.

## Requirements

1. **Follow `api/httpapi/apps.go`'s `AppsHandler` field for field.** The new handler carries a
   `txhelper.DB` pool, a `coredb.Querier`, a `func(pgx.Tx) coredb.Querier` defaulting to
   `coredb.New`, an `authz.Authorizer`, an `*observer.ObserverGroup`, and — the one addition — the
   `*fieldcrypto.Cipher`, for the post-rotation local reload. It has no `entityResolver` and no
   `typeResolver`: key rows are deliberately not entities and have no type slug. Export
   `NewFieldCryptoKeyHandler(...)` and `RegisterFieldCryptoKeyRoutes(r chi.Router, h *FieldCryptoKeyHandler)`.
2. **Register onto the shared `/v1` group** exactly as `RegisterAppRoutes` does, so the paths resolve
   under `/v1` without a second `Mount`. A top-level mount would reintroduce the documented
   duplicate-prefix panic.
3. **Four routes**, per the table in
   [`../notes/rotation-api-shape.md`](../notes/rotation-api-shape.md#routes):

   | Method | Path | Purpose | Success |
   | --- | --- | --- | --- |
   | `GET` | `/v1/field-crypto-keys` | Inventory of every version and its lifecycle timestamps | 200 |
   | `POST` | `/v1/field-crypto-keys/rotations` | Rotate — standard or compromised | 201 |
   | `POST` | `/v1/field-crypto-keys/{version}/mark-compromised` | Flag an already-retired key after the fact | 200 |
   | `PUT` | `/v1/field-crypto-keys/{version}/grace` | Extend, shorten, or clear a retired key's decrypt window | 200 |

4. **Key material never crosses the HTTP boundary.** The inventory route is served by
   `ListFieldCryptoKeyMetadata`, whose narrower column list yields a distinct sqlc row type carrying
   no `KeyBytes` field. No response body from any of the four routes may contain key bytes, hex, or
   any derivative of them.
5. **Rotation request handling.** Body `{ "key_hex": "…", "compromised": false,
   "grace_period_days": 30 }`:
   - `key_hex` is **optional**; omitted means the server generates 32 cryptographically random
     bytes (the recommended default). Supplied means bring-your-own-key, decoded and
     length-validated exactly as `CORE_FIELD_KEY_HEX` is. This field is deliberately retained — the
     user confirmed keeping it for HSM-derived and compliance-escrow operators.
   - `compromised` defaults to `false`; `true` stamps `compromised_at` on the key being retired.
   - `grace_period_days` is an optional non-negative integer; `null`/omitted means no expiry, which
     is the schema's deliberate safe default. **Combining it with `compromised: true` is rejected
     with 400**, naming `grace_period_days` — not silently overridden to `NULL`. An operator who
     typed both has a model of the system that disagrees with reality, and a 400 corrects it. The
     supported way to put a deadline on a compromised key is the `grace` route afterwards,
     deliberately and visibly.
6. **The rotation transaction** is the schema note's verbatim two statements in one `txhelper.Run`,
   in the mandatory order: `RetireActiveFieldCryptoKey` then `InsertActiveFieldCryptoKey`. The order
   is not negotiable — the one-active partial unique index is checked immediately and, being partial,
   cannot be deferred, so inserting before retiring always fails.
7. **Post-commit `h.cipher.Reload(r.Context())`** so the process that served the rotation converges
   immediately rather than waiting out the key-set TTL. A reload error is **logged and does not fail
   the request**: the rotation is already durable.
8. **Status mapping**, per the design note's table:
   - no actor on context (`opctx.ActorEntityID` miss) → 401;
   - authorization denied → 403;
   - malformed body, bad `key_hex`, negative days, or `compromised` + `grace_period_days` together
     → 400 `invalid_input` with a `FieldError` naming the offending field;
   - `RetireActiveFieldCryptoKey` returning zero rows (a lost concurrent rotation, or no active key)
     → 409 `apiresp.Conflict(...)`. This is what turns the "one rotation wins, the other errors and
     is retried" story into something actionable for an operator instead of a 500;
   - `key_bytes` unique violation (operator re-supplied material already on file — notably a key
     retired as compromised) → 409, with a message naming the cause;
   - anything else → 500.
9. **`mark-compromised`** takes an empty body and calls `MarkFieldCryptoKeyCompromised`, which is
   idempotent by query construction (`COALESCE(compromised_at, now())`), so a repeat call returns 200
   with the original timestamp. Zero rows means either an unknown version or the still-active key;
   distinguish them with one lookup against the inventory query and return **404** for unknown and
   **409** for the active key, with a message naming `POST /v1/field-crypto-keys/rotations` as the
   action to take instead.
10. **`grace`** takes `{ "grace_period_days": 30 }` or `{ "grace_period_days": null }` to clear the
    deadline, and calls `SetFieldCryptoKeyDecryptableUntil`. Split 404/409 on zero rows the same way.
11. **Authorization: every route uses `h.az.Authorize(r.Context(), "manage", nil)`** — including the
    read-only inventory route. Two deliberate choices to preserve:
    - `"manage"` rather than a new `"rotate"` verb, because operation slugs are registered in
      mod-authz (another repository, out of scope), and an unregistered slug falls through to a
      wildcard-`manage` check anyway;
    - `"manage"` rather than `"list"`/`"read"` for the inventory, because it discloses the full
      rotation history and which keys are flagged compromised — security-operational information with
      no reason to be readable by anyone who cannot rotate.

    A nil target is denied for every actor except one holding a wildcard grant, so this is
    fail-closed by construction and is mod-core's established admin-only idiom.
12. **Audit the rotation itself.** Unlike the read-path write-back, an admin rotation *is* a
    domain-significant, security-relevant mutation: dispatch it to the observer group inside the
    rotation transaction as `"rotate"` on resource `field_crypto_key` with a `nil` target, with
    `before`/`after` carrying versions, `retired_at`, `compromised_at`, and `decryptable_until` —
    and **never `key_bytes`**. `mark-compromised` and `grace` observe as `"update"` on the same
    resource string.
13. **Tests**, following `api/httpapi/apps_test.go`'s style: the 401/403 gates on every route, the
    400 on `compromised` + `grace_period_days`, the 400 on a malformed `key_hex`, the 409 on a
    zero-row retire, the 404/409 split on `mark-compromised` and `grace`, idempotency of
    `mark-compromised`, and an assertion that no response body from any route contains key material.

## Validation

- `cd api && make build`, `cd api && make test`, and `cd api && make lint` all pass.
- `cd api && go test ./httpapi/... -run FieldCryptoKey -v` shows the full case list passing.
- `grep -n "key_bytes\|KeyBytes" api/httpapi/field_crypto_keys.go` returns nothing outside a comment
  explaining the exclusion.
- `grep -c "Authorize(" api/httpapi/field_crypto_keys.go` shows one authorization call per route
  handler, all using `"manage"` with a nil target.
- `grep -n "Reload(" api/httpapi/field_crypto_keys.go` shows the post-commit reload, and a test
  asserts a reload failure does not fail the request.
- `grep -n "RetireActiveFieldCryptoKey" -A6 api/httpapi/field_crypto_keys.go` shows
  `InsertActiveFieldCryptoKey` following it inside the same transaction closure.
- `git diff --stat` shows exactly two new files; `moduleforge.module.yaml` and
  `api/openapi.fragment.yaml` are untouched by this task.

## Metadata

architectural_impact: true

## Assumptions

- Phase 1's five admin-facing queries and Phase 2's exported `Cipher.Reload` are available.
- `apiresp` provides `Conflict(...)`, `InvalidInput(...)`, and the `FieldError` type; check the
  actual constructor names in `api/apiresp/` and use them rather than the illustrative forms here.
- `ObserverGroup.Observe` accepts a `*int64` target, so `nil` is well-formed for a non-entity
  resource. Verify the actual signature in `api/observer/`.
- Phase 3 need not have landed — this handler depends only on Phases 1 and 2. If the phases run in
  order it will simply already be there.

## References

- [`../notes/rotation-api-shape.md`](../notes/rotation-api-shape.md#b-admin-rotation-api) — the
  authoritative design: routes, request/response shapes, status mapping, handler shape,
  authorization rationale, and auditing. Read the whole of section B, plus
  [Schema open question 5](../notes/rotation-api-shape.md#schema-open-question-5--does-a-compromised-rotation-force-decryptable_until--null).
- [`../notes/key-store-schema-design.md`](../notes/key-store-schema-design.md#rotation-transaction) —
  the transaction ordering and its concurrency behavior.
- `api/httpapi/apps.go` and `api/httpapi/apps_test.go` — the shape and test style to follow.

## Checkpoint hints

- After the handler struct, constructor, route registration, and the inventory `GET` with its authz
  gate.
- After the rotation `POST` including validation, transaction, and status mapping.
- After `mark-compromised` and `grace` with their 404/409 split.
- After the observer dispatch and the full test file.

## Status

**succeeded** — 2026-08-13.

Files: [`api/httpapi/field_crypto_keys.go`](../../api/httpapi/field_crypto_keys.go) (new),
[`api/httpapi/field_crypto_keys_test.go`](../../api/httpapi/field_crypto_keys_test.go) (new). No
other file changed: `moduleforge.module.yaml` and `api/openapi.fragment.yaml` are untouched, as task
002 owns them.

Validation: `cd api && make build`, `make test`, and `make lint` all pass;
`go test ./httpapi/ -run FieldCryptoKey -v` passes 26 test functions (48 run entries with subtests),
also green under `-race`. All six grep checks pass — `key_bytes`/`KeyBytes` appears only in one comment
explaining the exclusion, `Authorize(` appears exactly four times (one per route, all
`"manage"` with a nil target), `Reload(` appears once post-commit, and
`InsertActiveFieldCryptoKey` follows `RetireActiveFieldCryptoKey` inside the same closure.

Decisions taken where the task doc left latitude:

- Zero-row detection is `pgx.ErrNoRows` (what a sqlc `:one` returns for an UPDATE that matched
  nothing), classified into the `errNoActiveFieldCryptoKey` sentinel at the retire step rather than
  inferred from the transaction error at the top level.
- `mark-compromised` and `grace` run their one inventory lookup *before* the update, inside the same
  transaction: it splits 404 from 409 off one consistent snapshot and supplies the observer's
  `before` state without a second query.
- The rotation transaction reads the inventory once after its two statements, because `retired_at`
  and `decryptable_until` are resolved by the database clock and cannot be reconstructed in Go.
- Request bodies reject unknown members: a misspelled `compromised` would otherwise perform a
  standard rotation while the operator believed they had flagged a leaked key. An absent body is the
  zero request on `rotations` (the recommended invocation) but a 400 on `grace`, where reading it as
  "clear the deadline" would let a truncated request silently drop an expiry.
- `key_hex` is additionally rejected when all-zero, matching `CORE_FIELD_KEY_HEX`'s own guard: such a
  row would be persisted as the active key and only then refused when a cipher tried to build an
  AEAD from it. `grace_period_days` is bounded to `int32` so it cannot wrap to a past deadline.
