package httpapi

import (
	"context"
	"log/slog"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/moduleforge/core-api/audit"
	"github.com/moduleforge/core-api/service"
)

// TxBeginner abstracts transaction creation so handlers are testable without
// a real database. *pgxpool.Pool satisfies this interface.
type TxBeginner interface {
	Begin(ctx context.Context) (pgx.Tx, error)
}

// Deps carries the external dependencies that httpapi handlers need.
// All fields are required except Tx, which falls back to Pool if nil.
type Deps struct {
	// Pool is the pgxpool used to derive tx-scoped queriers for multi-row creates.
	Pool *pgxpool.Pool
	// Tx overrides Pool for transaction creation. Used in tests with a fake.
	// If nil, Pool is used directly.
	Tx TxBeginner
	// Services holds the entity CRUD implementations.
	Services *service.Services
	// Audit is the consumer-provided audit writer (also held by Services, but
	// exposed here for handlers that need to write directly, e.g. putSelf).
	Audit audit.Writer
	// Principal extracts caller identity from request context.
	Principal service.PrincipalExtractor
	// Logger is the structured logger for handler-level error messages.
	Logger *slog.Logger
}

// txBeginner returns the TxBeginner to use for starting transactions.
func (d *Deps) txBeginner() TxBeginner {
	if d.Tx != nil {
		return d.Tx
	}
	return d.Pool
}

type handlers struct {
	d Deps
}

// NewRouter wires all /entities/* routes and returns a mountable chi.Router.
// Mount it under any prefix, e.g. r.Mount("/v1", core.NewRouter(deps)).
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

		// Generic entity routes — must come after typed sub-paths to avoid
		// capturing /natural-persons etc.
		r.Get("/{uuid}", h.getEntity)
		r.Delete("/{uuid}", h.archiveEntity)
	})

	return r
}
