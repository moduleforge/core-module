package service

import (
	"github.com/jackc/pgx/v5"

	"github.com/moduleforge/core-api/authz"
	"github.com/moduleforge/core-api/entity"
	"github.com/moduleforge/core-api/internal/fieldcrypto"
	"github.com/moduleforge/core-api/observer"
	"github.com/moduleforge/core-api/txhelper"
	"github.com/moduleforge/core-api/types"
	coredb "github.com/moduleforge/core-model/db"
)

// Services is the aggregate of all entity service implementations. Consumers
// construct this once at startup and pass it into httpapi.NewRouter.
type Services struct {
	Entity         EntityServicer
	NaturalPerson  NaturalPersonServicer
	Corporation    CorporationServicer
	ServiceAccount ServiceAccountServicer

	// q is the base Querier backed by the pool, exposed so handlers can
	// derive tx-scoped queriers via coredb.New(tx).
	q coredb.Querier
}

// New constructs a Services aggregate.
//
// q is typically coredb.New(pool) and is used as the base querier for reads;
// mutations open their own transactions via db.
//
// db is the connection pool (or any txhelper.DB) used to open transactions for
// mutating operations.
//
// az gates every operation; a non-nil error from az.Authorize aborts the
// operation immediately.
//
// obs receives in-tx and post-commit notifications for every mutation;
// pass observer.NewObserverGroup() for a no-op group.
//
// cipher is used to encrypt and decrypt SSN and EIN fields; it must not be nil.
// New wraps it internally in a RotatingCipher (write-back handle db, default
// logger) so every decrypt of a blob written under a retired key re-encrypts
// and persists it under the active key on read; callers of New are unaffected.
//
// entityResolver maps public UUIDs to internal entity IDs; use entity.NewResolver()
// for the default 403-on-missing policy.
//
// typeResolver maps resource slugs to internal type IDs; must be pre-populated
// via types.New at startup.
func New(
	q coredb.Querier,
	db txhelper.DB,
	az authz.Authorizer,
	obs *observer.ObserverGroup,
	cipher *fieldcrypto.Cipher,
	entityResolver *entity.Resolver,
	typeResolver *types.Resolver,
) *Services {
	newQ := func(tx pgx.Tx) coredb.Querier { return coredb.New(tx) }
	rc := NewRotatingCipher(cipher, db, nil)
	return &Services{
		Entity:         &EntityService{db: db, az: az, obs: obs, newQuerier: newQ, entityResolver: entityResolver},
		NaturalPerson:  &NaturalPersonService{db: db, az: az, obs: obs, cipher: rc, newQuerier: newQ, entityResolver: entityResolver, typeResolver: typeResolver},
		Corporation:    &CorporationService{db: db, az: az, obs: obs, cipher: rc, newQuerier: newQ, entityResolver: entityResolver, typeResolver: typeResolver},
		ServiceAccount: &ServiceAccountService{db: db, az: az, obs: obs, newQuerier: newQ, entityResolver: entityResolver, typeResolver: typeResolver},
		q:              q,
	}
}

// Querier returns the base Querier so handlers can derive tx-scoped variants
// via coredb.New(tx) for multi-table creates.
func (s *Services) Querier() coredb.Querier {
	return s.q
}

// NaturalPersonFromServices extracts the NaturalPersonServicer from a Services
// aggregate. Used by the generated wiring to inject the NaturalPerson service
// into modules that require it without constructing a second Services instance.
func NaturalPersonFromServices(svcs *Services) NaturalPersonServicer {
	return svcs.NaturalPerson
}
