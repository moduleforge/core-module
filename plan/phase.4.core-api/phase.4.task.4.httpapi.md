# Phase 4, Task 4 — httpapi router + handlers

## Context
Consumers mount `core.NewRouter(deps)` into their chi tree. Handlers are thin: decode → authorize via PrincipalExtractor → call service → encode.

## Acceptance
- File `core-module/api/httpapi/router.go`:
  ```go
  package httpapi

  import (
      "log/slog"
      "net/http"
      "github.com/go-chi/chi/v5"
      "github.com/jackc/pgx/v5/pgxpool"
      "github.com/moduleforge/core-api/audit"
      "github.com/moduleforge/core-api/service"
  )

  type Deps struct {
      Pool      *pgxpool.Pool
      Services  *service.Services
      Audit     audit.Writer        // passed into services at construction; kept here for handlers that need it directly
      Principal service.PrincipalExtractor
      Logger    *slog.Logger
  }

  func NewRouter(d Deps) chi.Router {
      r := chi.NewRouter()
      h := &handlers{d: d}
      r.Route("/entities", func(r chi.Router) {
          r.Get("/self", h.getSelf)
          r.Put("/self", h.putSelf)
          r.Post("/natural-persons", h.createNaturalPerson)
          r.Get("/natural-persons/{uuid}", h.getNaturalPerson)
          r.Put("/natural-persons/{uuid}", h.updateNaturalPerson)
          r.Post("/corporations", h.createCorporation)
          r.Get("/corporations/{uuid}", h.getCorporation)
          r.Put("/corporations/{uuid}", h.updateCorporation)
          r.Post("/service-accounts", h.createServiceAccount)
          r.Get("/service-accounts/{uuid}", h.getServiceAccount)
          r.Put("/service-accounts/{uuid}", h.updateServiceAccount)
          r.Get("/{uuid}", h.getEntity)
          r.Delete("/{uuid}", h.archiveEntity)
      })
      return r
  }
  ```
- Handler files grouped by resource:
  - `self.go` — getSelf, putSelf.
  - `natural_persons.go` — create/get/update.
  - `corporations.go` — create/get/update.
  - `service_accounts.go` — create/get/update.
  - `entities.go` — getEntity (resolve subtype), archiveEntity.
- `response.go` — `jsonOK(w, status, body any)`, `jsonErr(w, status int, code, msg string)` helpers. Mirror the shape used by `users-module/api/internal/server`.
- Handler skeleton:
  ```go
  func (h *handlers) getSelf(w http.ResponseWriter, r *http.Request) {
      p, ok := h.d.Principal.FromContext(r.Context())
      if !ok {
          jsonErr(w, 401, "unauthorized", "auth required")
          return
      }
      resp, err := h.d.Services.Entity.GetSelf(r.Context(), h.d.Pool, *p)
      if err != nil {
          writeServiceErr(w, err)
          return
      }
      jsonOK(w, 200, resp)
  }
  ```
- `writeServiceErr` maps service sentinel errors → HTTP codes (`ErrNotFound`→404, `ErrForbidden`→403, `ErrInvalidInput`→400, default→500).

## How to verify
- `go build ./httpapi` exits 0.
- `go vet ./httpapi` exits 0.
- Handler tests in Task 4.5 cover happy path + 401 + 403 + 404.

## Notes
- For PUT payloads, validate required fields before calling the service; surface `ErrInvalidInput` for known bad input.
- Handlers must NOT open transactions themselves — for single-table updates the pool is passed as the DBTX; multi-table creates happen inside the service which uses the pool if no outer tx is supplied.
- When a consumer needs a cross-module tx, they bypass HTTP and call the service directly with their own tx — the handler path is for the single-module case.
