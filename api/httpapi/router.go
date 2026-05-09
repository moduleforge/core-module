package httpapi

import (
	"log/slog"

	"github.com/go-chi/chi/v5"

	"github.com/moduleforge/core-api/service"
)

// Deps carries the external dependencies that httpapi handlers need.
type Deps struct {
	// Services holds the entity CRUD implementations.
	Services *service.Services
	// Logger is the structured logger for handler-level error messages.
	Logger *slog.Logger
}

type handlers struct {
	d Deps
}

// NewRouter wires /entities/* routes and returns a mountable chi.Router.
// Mount it under any prefix, e.g. r.Mount("/v1", core.NewRouter(deps)).
//
// Core does not expose /self: that endpoint only makes sense for a
// consumer that has a users/authentication layer (which core does not).
// Consumers like users-module own /self and compose a response from
// their own user row plus core's EntityService.GetSelf helper.
func NewRouter(d Deps) chi.Router {
	r := chi.NewRouter()
	h := &handlers{d: d}

	r.Route("/entities", func(r chi.Router) {
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
