# Database considerations

This note records the architectural decisions behind the data layer — both the
choice of database engine and the choice of migration tooling — and the
alternatives that were considered and rejected. Future contributors should read
this before proposing a different DB engine, ORM, or migration tool.

## Decision: canonical state lives in Postgres

The system of record for canonical, transactional data is **Postgres**. This is
treated as a deliberate architectural commitment, not an incidental
implementation choice.

### Why Postgres specifically

- **SQL gives us efficient storage, mature tooling, and ACID guarantees.** For
  user accounts, audit logs, and the entity/legal-entity hierarchy this module
  is built around, ACID is non-negotiable. The SQL ecosystem (drivers, query
  generators, observability, BI tools) is the broadest and most stable in the
  industry.
- **Postgres is the most capable mainstream SQL implementation.** Several
  features it offers — most notably **partial indexes** and **`JSONB`** — are
  load-bearing in this codebase or are likely to become so. Replacing them
  would cost real work or real performance:
  - Partial indexes (`CREATE INDEX … WHERE …`) are already used for
    archived-row exclusion (`entities_archived_at_idx`,
    `contacts_legal_entity_idx`, etc.) and for compound-partial uniqueness on
    OIDC identity (`user_accounts_auth_idx`). Without them, every index
    scan would pay the cost of the archived/null subset.
  - `JSONB` (binary, indexable via GIN) is the natural representation for the
    audit-log row deltas required by this module's stated feature set
    ("any data change is logged"). MySQL `JSON` is text-backed and slower;
    SQLite stores it as text. No portable substitute matches its query
    performance.
  - `RETURNING` is used pervasively by the sqlc-generated query layer. Engines
    without it require an INSERT + SELECT round-trip, sacrificing both
    performance and correctness ergonomics around generated column values.
  - Other features we benefit from but use less centrally: `TIMESTAMPTZ`,
    native `UUID` type (16 bytes vs 36 in `CHAR(36)`), rich PL/pgSQL trigger
    support, `pgcrypto`, `DISTINCT ON`, `ON CONFLICT ... DO UPDATE`, and the
    GIN/GIST/BRIN index families.

### Where other databases fit

Postgres is the right choice for the canonical state. It is not necessarily
the right choice for everything:

- **Caches, ephemeral state, message buses, search indexes** — Redis,
  Elasticsearch, NATS, and similar tools may be layered on as needed without
  any conflict with this decision. The constraint is one-way: every other
  store derives from or is fed by Postgres.
- This separation lets each layer use the right tool for its job without
  diluting the canonical-state guarantees Postgres gives us.

### Implication: SQL written here may use Postgres-specific features

The earlier "vanilla SQL only" goal in the project's CLAUDE.md is **withdrawn**
in favor of this position. SQL in `migrations/` and `queries/` may use
Postgres-specific syntax, types, and functions where they materially help
clarity, performance, or correctness. Contributors should still prefer
standards-conformant SQL when the choice is otherwise neutral, but should not
contort the schema to maintain artificial portability.

## Alternatives considered and rejected

### Multi-DB schema-as-code via Ent

[Ent](https://entgo.io) (Go schema-as-code framework on top of Atlas) would
have given us tested multi-DB portability for the common case, with per-dialect
escape hatches for advanced features. **Rejected because:**

- The portability benefit is only valuable if we actually intend to run on
  multiple engines. Postgres is sufficient.
- The CTI / shared-PK inheritance pattern at the heart of this module
  (`entities → legal_entities → natural_persons / corporations`) is awkward in
  Ent's edge model and would require manual workarounds.
- Trading hand-authored SQL + sqlc (which we already use to good effect) for
  Go-defined schemas is a significant paradigm shift with no commensurate
  payoff while we're committed to a single engine.

### Common-denominator (lowest-feature) SQL

Hand-write only SQL that runs unchanged on Postgres, MySQL, and SQLite.
**Rejected because:**

- It pays real costs (no partial indexes, no `JSONB`, no `RETURNING`, no
  `TIMESTAMPTZ`, larger UUID storage) for theoretical portability that we are
  not exercising. Dev / CI / prod would all still run Postgres, so dialect
  drift would only be discovered if and when we ever tried to switch.
- The audit-log feature in particular is impractical without `JSONB` for row
  deltas.

### Liquibase changelogs (DSL with per-DB DDL emission)

A YAML/XML/JSON DSL that emits per-DB DDL automatically, with explicit
escape hatches for engine-specific SQL. **Rejected because:**

- Same root reason as Ent — we don't need the portability.
- Pulls in a JVM runtime dependency that nothing else in the stack uses.
- Has a freemium model similar to the migration tool we're replacing
  (see below), which contradicts the OSS-first preference.

### Atlas Pro `composite_schema` / `external_schema`

Use the paid Atlas Pro features that natively model cross-module schema
composition. **Rejected because:**

- Adds vendor lock-in to a paid product.
- Conflicts with the OSS-first preference.
- The free-tier composition pattern (Makefile-based directory merge) works
  fine for our needs.

## Decision: migration tooling is goose

Migrations are managed by [pressly/goose](https://github.com/pressly/goose).

### Why goose over Atlas

The previous tool was [Atlas](https://atlasgo.io) (Apache 2.0 OSS core with
proprietary Pro features). It worked well but did not earn its complexity:

- **OSS-first preference.** goose is fully MIT-licensed with no Pro tier.
  The features we'd most likely want next from Atlas (`composite_schema`,
  cloud integrations) are paywalled. We prefer a tool whose roadmap aligns
  with the OSS ethos rather than nudging users toward a paid offering.
- **Simpler, more targeted.** goose is a single Go binary with a small command
  surface (`goose up`, `goose status`, `goose create`, `goose validate`). No
  HCL configuration, no integrity-hash file, no shadow-DB orchestration baked
  into the tool. Each module's `Makefile` is shorter and easier to read.
- **Versioned migrations are the only model we use.** Atlas's declarative /
  diff modes were never employed here. goose only does versioned migrations,
  which is exactly what we want.
- **Integrity-hash file (`atlas.sum`) is unnecessary for our workflow.**
  Migrations are distributed as part of git-checked or Go-module-checked
  packages, so package-level integrity already covers the threat model.
- **Multi-DB support is preserved if ever needed.** goose supports Postgres,
  MySQL, SQLite, MSSQL, ClickHouse, and more — even though we're committing
  to Postgres, we don't lose option value.

### What we trade away

- **`atlas migrate lint` shadow-DB validation** is replaced by a small
  per-module Make target that spins up an ephemeral Postgres container and
  applies all migrations to it. Same effect, ~30 lines of shell, no tool
  dependency beyond Docker.
- **`atlas.sum` integrity-hash file** is dropped. Tampering protection now
  comes from package distribution (git, Go module proxy) rather than an
  in-tree checksum.

### Migration file conventions under goose

- Files use the existing 4-digit sequential prefix convention
  (`0008_entities.sql`, `0300_contacts.sql`, etc.). Each module reserves its
  own range:
  - `core-module`: `0001–0099` (goose reserves version 0)
  - `users-module`: `0100–0199`
  - `tags-module`: `0200–0299`
  - `contacts-module`: `0300–0399`
- Every file begins with `-- +goose Up`. Down sections are not provided —
  migrations are forward-only by convention.
- Any `CREATE [OR REPLACE] FUNCTION … $$ … $$ LANGUAGE …;` block (PL/pgSQL or
  SQL functions with internal semicolons) must be wrapped in
  `-- +goose StatementBegin` / `-- +goose StatementEnd` markers so goose
  treats it as a single statement.
- The cross-module composition mechanism (Makefile `compose` target that
  copies `core-module` migrations + the consumer module's own migrations into
  a unified `schema/migrations/` directory) is unchanged conceptually; only
  the tool that operates on the composed directory has changed.
