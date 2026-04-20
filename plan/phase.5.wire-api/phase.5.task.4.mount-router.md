# Phase 5, Task 4 — Mount core router in main.go

## Context
Replace the existing route registrations for `/v1/self` and entity-touching admin routes with a single `r.Mount` of core-module's subrouter.

## Acceptance
In `users-module/api/cmd/server/main.go`, inside the `/v1` protected group:

```go
coreSvcs := coreservice.New(pool, auditWriter)
coreRouter := corehttpapi.NewRouter(corehttpapi.Deps{
    Pool:      pool,
    Services:  coreSvcs,
    Audit:     auditWriter,
    Principal: auth.CorePrincipalAdapter{},
    Logger:    logger,
})

r.Route("/v1", func(r chi.Router) {
    r.Use(requireConfirmed)
    r.Group(func(r chi.Router) {
        r.Use(auth.RequireAuth(jwtValidator))
        // Core entity routes (including /self)
        r.Mount("/", coreRouter)
        // Users-module routes that remain
        r.Group(func(r chi.Router) {
            r.Use(auth.RequireAdmin)
            r.Post("/users", usersHandler.Create)
            // ... other admin routes
        })
        r.Route("/users/{uuid}", func(r chi.Router) {
            // non-admin-gated user routes
        })
        // ... apps, audit, assume, etc.
    })
})
```

- Remove all `r.Get("/self", selfHandler.Get)` / `r.Put("/self", selfHandler.Put)` and any users-module route that duplicates a core route (natural-persons admin CRUD if it existed as a separate route — currently it's embedded in `/v1/users`, which will be reworked in 5.5 rather than removed here).

## How to verify
- `go build ./...` exits 0.
- Spin up locally; `curl /v1/self` hits core's handler (observable via added log line or via correct response shape).
- No route conflicts reported by chi.

## Notes
- Order matters: core router mounted at `/` within `/v1` — make sure users-module's `/users/*` routes are registered AFTER so chi's routing tree resolves correctly. Actually chi resolves by exact path match before mount — no conflict expected, but verify.
- If chi's mount behavior causes `/v1/users` to be shadowed, mount core at a narrower path (e.g. `/entities`) and update the route tables + OpenAPI fragment.
