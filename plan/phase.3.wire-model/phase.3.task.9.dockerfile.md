# Phase 3, Task 9 — Update users-module/api Dockerfile

## Context
The Docker build context is users-module/; core migrations live outside it. Solution: run `go mod download` during the Docker build so core-model lands in `$GOMODCACHE`, then copy migrations from there into the final image.

## Acceptance
- `users-module/api/Dockerfile` includes:
  ```dockerfile
  FROM golang:1.23 AS build
  WORKDIR /src
  COPY go.mod go.sum ./
  RUN go mod download
  # locate core-model's migrations and stage them
  RUN cp -r $(go env GOMODCACHE)/github.com/moduleforge/core-model@*/migrations /tmp/core-migrations
  COPY ../users-module/model/migrations ./migrations
  RUN mkdir -p /app/schema/migrations \
      && cp /tmp/core-migrations/*.sql /app/schema/migrations/ \
      && cp ./migrations/*.sql /app/schema/migrations/
  # ... build continues
  ```
  (Adjust to fit the existing Dockerfile structure.)
- Final image has `/app/schema/migrations/` containing all composed SQL files in correct order.
- `docker build` from users-module root succeeds.

## How to verify
- `cd users-module && docker build -f api/Dockerfile -t users-api:test .`
- `docker run --rm users-api:test ls /app/schema/migrations/` shows 6 core + 10 users-module files.

## Notes
- If the Dockerfile currently relies on ko (per users-module/api/.ko.yaml), the migration staging logic may need to live in ko's base-image step or a separate runtime image. Check with the users-module/plan/phase.8 context first.
- If build context must include the outer repo, that's an alternative — document whichever path is chosen.
