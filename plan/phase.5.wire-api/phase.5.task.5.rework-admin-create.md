# Phase 5, Task 5 — Rework admin user-create in handlers/users.go

## Context
Current flow (`users-module/api/internal/handlers/users.go:79–105`) writes entity + legal_entity + natural_person + user + auth_local directly in a single tx using coredb + users-module db. Move the entity chain into a call to core service while keeping the outer tx in users-module's handler.

## Acceptance
```go
func (h *UsersHandler) Create(w http.ResponseWriter, r *http.Request) {
    // decode request
    // authorize (admin only — already gated by middleware)
    tx, err := h.pool.Begin(r.Context())
    if err != nil { /* 500 */ return }
    defer tx.Rollback(r.Context())

    actor := auth.MustFromContext(r.Context())  // users-module context
    corePrin := service.Principal{UserID: actor.UserID, EntityID: actor.EntityID, IsAdmin: actor.IsAdmin}

    // Delegate entity chain to core service
    np, entityUUID, err := h.coreSvcs.NaturalPerson.Create(r.Context(), tx, corePrin, service.CreateNaturalPersonInput{
        GivenName:  req.GivenName,
        FamilyName: req.FamilyName,
    })
    if err != nil { /* map to status */ return }

    // Insert users row
    user, err := h.q.WithTx(tx).CreateUser(r.Context(), db.CreateUserParams{
        EntityID: np.EntityID,
        Email:    req.Email,
        IsAdmin:  req.IsAdmin,
        // ...
    })
    if err != nil { /* map */ return }

    // Optional auth_local
    if req.Password != nil {
        hash, _ := auth.HashPassword(*req.Password)
        _, err := h.q.WithTx(tx).CreateAuthLocal(r.Context(), db.CreateAuthLocalParams{UserID: user.ID, PasswordHash: hash})
        if err != nil { /* map */ return }
    }

    if err := tx.Commit(r.Context()); err != nil { /* 500 */ return }
    // encode response
}
```

- Remove the coredb.* calls that currently live in `users.go`; they're now behind the service.
- `UsersHandler` struct gains a `coreSvcs *service.Services` field, injected via constructor from main.go.
- Get/Update/Delete admin paths for users (that read/write natural_person fields) similarly delegate to core services.

## How to verify
- `go build` exits 0.
- Existing `TestUsersHandler_Create_*` tests still pass (adjust mocks to include core service).
- Manual smoke: POST /v1/users with a new admin body creates entity + user + auth_local atomically; rollback on any step failure.

## Notes
- Keep audit writes on the user row (in users-module) distinct from audit writes inside the core service (for entity/natural_person). Both fire in the same request → two audit rows, which correctly reflects the two resources.
