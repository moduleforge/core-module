package service

import (
	"github.com/moduleforge/core-api/audit"
	coredb "github.com/moduleforge/core-model/db"
)

// Services is the aggregate of all entity service implementations. Consumers
// construct this once at startup and pass it into httpapi.NewRouter.
type Services struct {
	Entity         EntityServicer
	LegalEntity    LegalEntityServicer
	NaturalPerson  NaturalPersonServicer
	Corporation    CorporationServicer
	ServiceAccount ServiceAccountServicer

	// q is the base Querier backed by the pool, exposed so handlers can
	// derive tx-scoped queriers via coredb.New(tx).
	q coredb.Querier
}

// New constructs a Services aggregate. q is typically coredb.New(pool) and is
// used as the base querier; callers may pass a tx-scoped querier to individual
// service methods for multi-table atomicity.
func New(q coredb.Querier, aw audit.Writer) *Services {
	return &Services{
		Entity:         &EntityService{aw: aw},
		LegalEntity:    &LegalEntityService{},
		NaturalPerson:  &NaturalPersonService{aw: aw},
		Corporation:    &CorporationService{aw: aw},
		ServiceAccount: &ServiceAccountService{aw: aw},
		q:              q,
	}
}

// Querier returns the base Querier so handlers can derive tx-scoped variants
// via coredb.New(tx) for multi-table creates.
func (s *Services) Querier() coredb.Querier {
	return s.q
}
