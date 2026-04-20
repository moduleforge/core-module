# Phase 4, Task 6 — OpenAPI fragment

## Context
users-module ships a full OpenAPI spec at `users-module/api/openapi.yaml`. core-module owns the `/entities/*` routes and should publish a reusable fragment that users-module composes into its spec via `$ref`.

## Acceptance
- File `core-module/api/openapi.fragment.yaml` — an OpenAPI 3.0.3 document containing:
  - `paths:` for all routes listed in phase.4.core-api.md (the routing table).
  - `components/schemas:` for request/response bodies: `Entity`, `LegalEntity`, `NaturalPerson`, `Corporation`, `ServiceAccount`, `ProfileResponse`, `UpdateProfileRequest`, `CreateNaturalPersonRequest`, etc.
  - `components/responses:` for shared error shapes (`Unauthorized`, `Forbidden`, `NotFound`, `BadRequest`).
  - `security:` references a bearer-auth scheme named `bearerAuth` defined in `components/securitySchemes`.
- Fragment lints clean: `npx @redocly/cli lint openapi.fragment.yaml`.
- users-module/api/openapi.yaml (Phase 5 will update) references the fragment via `$ref: '../../core-module/api/openapi.fragment.yaml#/paths'` or a similar mechanism.

## How to verify
- Lint passes.
- `openapi.fragment.yaml` can be rendered standalone by `npx @redocly/cli preview-docs openapi.fragment.yaml` (as a sanity check).

## Notes
- Don't duplicate auth_local/apps/users schemas — those stay in users-module's spec.
- The fragment can omit `servers:` since it's meant to be composed.
