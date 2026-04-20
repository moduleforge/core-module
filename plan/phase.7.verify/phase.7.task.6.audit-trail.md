# Phase 7, Task 6 — Audit trail verification

## Acceptance
After a profile-edit smoke test:
- `psql $DB_URL -c "SELECT op, resource, actor_user_id FROM audit_log ORDER BY at DESC LIMIT 5;"` shows a row with `op='update'`, `resource='natural_person'`, `actor_user_id` = the user who edited.
- `before` / `after` JSON columns are populated with the old and new given_name/family_name.

## How to verify
Run the SQL.

## Notes
- If actor_user_id is null, the PrincipalExtractor or audit writer adapter is broken — go back to Phase 5.
