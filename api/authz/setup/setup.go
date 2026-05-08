// Package setup provides utilities for generating and applying SQL access
// policy functions at application startup.
//
// Access policy functions have the signature:
//
//	accessible_<slug>_ids_for_actor(p_actor_entity_id BIGINT)
//	  RETURNS TABLE(entity_id BIGINT) LANGUAGE sql STABLE
//
// These functions are referenced by list/search queries as JOIN targets.
// Because function bodies vary by Authorizer implementation, they are
// generated at runtime via CREATE OR REPLACE FUNCTION DDL rather than being
// baked into migration history.
package setup

import (
	"context"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
)

// AccessFuncGenerator produces the SQL body for the access function targeting
// a given resource slug. The body is the SELECT statement only — the CREATE
// OR REPLACE FUNCTION wrapper is assembled by GenerateFuncs.
type AccessFuncGenerator interface {
	GenerateForResource(slug string) (string, error)
}

// GenerateFuncs assembles a single SQL string containing CREATE OR REPLACE
// FUNCTION statements for all the supplied resource slugs. Each function has
// the signature:
//
//	accessible_<slug>_ids_for_actor(p_actor_entity_id BIGINT)
//	  RETURNS TABLE(entity_id BIGINT) LANGUAGE sql STABLE
//
// The body of each function is obtained from gen. Slugs are processed in the
// order provided. An error from any generator call aborts and returns that
// error.
func GenerateFuncs(gen AccessFuncGenerator, slugs []string) (string, error) {
	var sb strings.Builder
	for i, slug := range slugs {
		body, err := gen.GenerateForResource(slug)
		if err != nil {
			return "", fmt.Errorf("setup.GenerateFuncs: generate body for %q: %w", slug, err)
		}

		if i > 0 {
			sb.WriteString("\n\n")
		}

		fmt.Fprintf(&sb,
			"CREATE OR REPLACE FUNCTION accessible_%s_ids_for_actor(p_actor_entity_id BIGINT)\n"+
				"RETURNS TABLE(entity_id BIGINT) LANGUAGE sql STABLE AS $$\n"+
				"%s\n"+
				"$$;",
			slug, body,
		)
	}
	return sb.String(), nil
}

// ApplyFuncs generates the CREATE OR REPLACE FUNCTION DDL for all slugs and
// executes it inside a single transaction. The operation is idempotent: a
// subsequent call with the same generator and slugs replaces the existing
// function definitions with identical ones.
func ApplyFuncs(ctx context.Context, db *pgxpool.Pool, gen AccessFuncGenerator, slugs []string) error {
	sql, err := GenerateFuncs(gen, slugs)
	if err != nil {
		return fmt.Errorf("setup.ApplyFuncs: generate: %w", err)
	}

	tx, err := db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("setup.ApplyFuncs: begin tx: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck // rollback on any path that doesn't commit

	if _, err := tx.Exec(ctx, sql); err != nil {
		return fmt.Errorf("setup.ApplyFuncs: exec DDL: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("setup.ApplyFuncs: commit: %w", err)
	}
	return nil
}

// PermissiveGenerator returns an AccessFuncGenerator that produces a body of
// the form:
//
//	SELECT entity_id FROM <table>
//
// for each resource slug, where <table> is looked up in tableForSlug. If a
// slug is not present in the map, GenerateForResource returns an error.
//
// Use this in test harnesses where authorization logic is not the focus and
// all entities should be accessible to any actor.
func PermissiveGenerator(tableForSlug map[string]string) AccessFuncGenerator {
	return &permissiveGen{tableForSlug: tableForSlug}
}

type permissiveGen struct {
	tableForSlug map[string]string
}

func (g *permissiveGen) GenerateForResource(slug string) (string, error) {
	table, ok := g.tableForSlug[slug]
	if !ok {
		return "", fmt.Errorf("permissive generator: no table mapping for slug %q", slug)
	}
	return fmt.Sprintf("SELECT entity_id FROM %s", table), nil
}

// DenyingGenerator returns an AccessFuncGenerator that produces a body of
// the form:
//
//	SELECT 0::BIGINT WHERE FALSE
//
// for every resource slug (returning the empty set). Use this in tests that
// verify deny paths — every access function returns no IDs, so all list
// queries return empty results.
func DenyingGenerator() AccessFuncGenerator {
	return &denyingGen{}
}

type denyingGen struct{}

func (*denyingGen) GenerateForResource(_ string) (string, error) {
	return "SELECT 0::BIGINT WHERE FALSE", nil
}
