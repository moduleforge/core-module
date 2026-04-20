# Phase 3, Task 4 — Renumber users-module migrations

## Context
users-module's own migrations currently run 0006–0015. Renumber to 0100+ so core's 0000–0005 sort first lexicographically in the composed dir.

## Mapping
```
0006_users.sql                        → 0100_users.sql
0007_auth_local.sql                   → 0101_auth_local.sql
0008_email_codes.sql                  → 0102_email_codes.sql
0009_password_resets.sql              → 0103_password_resets.sql
0010_apps.sql                         → 0104_apps.sql
0011_apps_users.sql                   → 0105_apps_users.sql
0012_audit_log.sql                    → 0106_audit_log.sql
0013_oidc_config.sql                  → 0107_oidc_config.sql
0014_oidc_providers.sql               → 0108_oidc_providers.sql
0015_drop_provider_enabled_json.sql   → 0109_drop_provider_enabled_json.sql
```

## Acceptance
- All renames done with `git mv` (preserves history).
- SQL content unchanged inside files.
- No references to the old filenames elsewhere in the repo.

## How to verify
- `ls users-module/model/migrations/` shows only 0100–0109 plus core (not yet — that's composed later).
- `git log --follow users-module/model/migrations/0100_users.sql` shows the history continuing from 0006_users.sql.
- `grep -rn "0006_users\|0007_auth_local" users-module/ core-module/` returns nothing.

## Notes
- Any internal SQL `COMMENT` or docstring referencing old migration numbers should be updated too.
