# Phase 5, Task 8 — Integration test for PUT /v1/self

## Context
End-to-end verification that the core handler, mounted via users-module, correctly updates a profile and triggers the users-module audit writer with the right actor.

## Acceptance
Test file `users-module/api/internal/handlers/self_integration_test.go` (or similar):

1. Spin up a test DB (reuse existing fixtures / testcontainer setup).
2. Create a test user via fixtures (admin + natural_person entity).
3. Obtain a JWT for the user.
4. `PUT /v1/self` with `{"given_name":"Updated"}` and the JWT.
5. Assert:
   - response 200.
   - DB `natural_persons.given_name` = "Updated".
   - `audit_log` has one row with `actor_user_id = testUser.id`, `target_entity_id = testUser.entity_id`, `op='update'`, `resource='natural_person'`.

## How to verify
- `make test` in users-module/api runs and passes this test.

## Notes
- If testcontainer setup doesn't exist, reuse whatever Phase 8 of users-module established — check `users-module/plan/phase.8.deploy-ci/`.
- This test is the golden-path proof that core + users-module composition works.
