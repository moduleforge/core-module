package setup_test

import (
	"strings"
	"testing"

	"github.com/moduleforge/core-api/authz/setup"
)

// --- GenerateFuncs tests ---

func TestGenerateFuncs_WellFormedDDL(t *testing.T) {
	t.Parallel()
	gen := setup.PermissiveGenerator(map[string]string{
		"natural_person": "natural_persons",
		"tag":            "tags",
	})
	slugs := []string{"natural_person", "tag"}

	sql, err := setup.GenerateFuncs(gen, slugs)
	if err != nil {
		t.Fatalf("GenerateFuncs: unexpected error: %v", err)
	}

	// Both function names must appear.
	for _, slug := range slugs {
		want := "CREATE OR REPLACE FUNCTION accessible_" + slug + "_ids_for_actor"
		if !strings.Contains(sql, want) {
			t.Errorf("expected DDL to contain %q; got:\n%s", want, sql)
		}
	}

	// Both functions must be terminated.
	if count := strings.Count(sql, "$$;"); count != len(slugs) {
		t.Errorf("expected %d function terminators ($$;), got %d; sql:\n%s", len(slugs), count, sql)
	}

	// Required signature components.
	if !strings.Contains(sql, "p_actor_entity_id BIGINT") {
		t.Errorf("expected parameter p_actor_entity_id BIGINT in DDL")
	}
	if !strings.Contains(sql, "p_op_ids INT[]") {
		t.Errorf("expected parameter p_op_ids INT[] in DDL")
	}
	if !strings.Contains(sql, "RETURNS TABLE(entity_id BIGINT)") {
		t.Errorf("expected RETURNS TABLE(entity_id BIGINT) in DDL")
	}
	if !strings.Contains(sql, "LANGUAGE sql STABLE") {
		t.Errorf("expected LANGUAGE sql STABLE in DDL")
	}
}

func TestGenerateFuncs_SingleSlug(t *testing.T) {
	t.Parallel()
	gen := setup.PermissiveGenerator(map[string]string{"tag": "tags"})
	sql, err := setup.GenerateFuncs(gen, []string{"tag"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.HasPrefix(sql, "CREATE OR REPLACE FUNCTION accessible_tag_ids_for_actor") {
		prefix := sql
		if len(prefix) > 80 {
			prefix = prefix[:80]
		}
		t.Errorf("unexpected SQL prefix: %q", prefix)
	}
}

func TestGenerateFuncs_EmptySlugs(t *testing.T) {
	t.Parallel()
	gen := setup.PermissiveGenerator(nil)
	sql, err := setup.GenerateFuncs(gen, nil)
	if err != nil {
		t.Fatalf("unexpected error for empty slugs: %v", err)
	}
	if sql != "" {
		t.Errorf("expected empty string for empty slugs; got %q", sql)
	}
}

func TestGenerateFuncs_GeneratorError(t *testing.T) {
	t.Parallel()
	// PermissiveGenerator returns an error for unmapped slugs.
	gen := setup.PermissiveGenerator(map[string]string{"tag": "tags"})
	_, err := setup.GenerateFuncs(gen, []string{"unknown_slug"})
	if err == nil {
		t.Fatal("expected error for unmapped slug; got nil")
	}
}

func TestGenerateFuncs_MultipleSlugsSeparated(t *testing.T) {
	t.Parallel()
	gen := setup.PermissiveGenerator(map[string]string{
		"natural_person":  "natural_persons",
		"tag":             "tags",
		"service_account": "service_accounts",
	})
	slugs := []string{"natural_person", "tag", "service_account"}
	sql, err := setup.GenerateFuncs(gen, slugs)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Functions should be separated by a blank line.
	if !strings.Contains(sql, "\n\nCREATE OR REPLACE FUNCTION") {
		t.Error("expected blank-line separator between function definitions")
	}
}

// --- PermissiveGenerator tests ---

func TestPermissiveGenerator_Body(t *testing.T) {
	t.Parallel()
	gen := setup.PermissiveGenerator(map[string]string{"tag": "tags"})
	body, err := gen.GenerateForResource("tag")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := "SELECT entity_id FROM tags"
	if body != want {
		t.Errorf("body = %q; want %q", body, want)
	}
}

func TestPermissiveGenerator_UnknownSlugErrors(t *testing.T) {
	t.Parallel()
	gen := setup.PermissiveGenerator(map[string]string{"tag": "tags"})
	_, err := gen.GenerateForResource("nonexistent")
	if err == nil {
		t.Fatal("expected error for unmapped slug; got nil")
	}
}

func TestPermissiveGenerator_MultipleResources(t *testing.T) {
	t.Parallel()
	tableMap := map[string]string{
		"natural_person": "natural_persons",
		"corporation":    "corporations",
		"tag":            "tags",
	}
	gen := setup.PermissiveGenerator(tableMap)
	for slug, table := range tableMap {
		body, err := gen.GenerateForResource(slug)
		if err != nil {
			t.Errorf("slug %q: unexpected error: %v", slug, err)
			continue
		}
		want := "SELECT entity_id FROM " + table
		if body != want {
			t.Errorf("slug %q: body = %q; want %q", slug, body, want)
		}
	}
}

// --- DenyingGenerator tests ---

func TestDenyingGenerator_Body(t *testing.T) {
	t.Parallel()
	gen := setup.DenyingGenerator()
	cases := []string{"natural_person", "corporation", "tag", "service_account", "any_slug"}
	for _, slug := range cases {
		t.Run(slug, func(t *testing.T) {
			t.Parallel()
			body, err := gen.GenerateForResource(slug)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			want := "SELECT 0::BIGINT WHERE FALSE"
			if body != want {
				t.Errorf("body = %q; want %q", body, want)
			}
		})
	}
}

// --- GrantTableGenerator tests ---

func TestGrantTableGenerator_Tag(t *testing.T) {
	t.Parallel()
	gen := setup.NewGrantTableGenerator()
	body, err := gen.GenerateForResource("tag")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Must contain the CTE skeleton.
	if !strings.Contains(body, "ActorChain") {
		t.Errorf("body missing ActorChain CTE; got:\n%s", body)
	}
	if !strings.Contains(body, "TargetChain") {
		t.Errorf("body missing TargetChain CTE; got:\n%s", body)
	}
	if !strings.Contains(body, "p_op_ids") {
		t.Errorf("body missing p_op_ids reference; got:\n%s", body)
	}
	// Type-scoped own semantics: the generic own-arm keys on owner_id, scoped to
	// the tag type. The subject_id access predicate has been dropped (subject_id
	// remains a data column; it is no longer used for access), and the body no
	// longer references the downstream tags table.
	if !strings.Contains(body, "type_is_or_descends_from(e.fundamental_type_id, 'tag')") {
		t.Errorf("tag body missing type predicate; got:\n%s", body)
	}
	if !strings.Contains(body, "e.owner_id = p_actor_entity_id") {
		t.Errorf("tag body missing owner_id own-clause; got:\n%s", body)
	}
	if strings.Contains(body, "subject_id") {
		t.Errorf("tag body must not reference subject_id (access predicate dropped); got:\n%s", body)
	}
	if strings.Contains(body, "FROM tags") {
		t.Errorf("tag body must not reference the downstream tags table; got:\n%s", body)
	}
}

func TestGrantTableGenerator_NaturalPerson(t *testing.T) {
	t.Parallel()
	gen := setup.NewGrantTableGenerator()
	body, err := gen.GenerateForResource("natural_person")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(body, "ActorChain") {
		t.Errorf("body missing ActorChain CTE; got:\n%s", body)
	}
	// Own predicate: owner_id = actor (owns-itself convention), scoped to type.
	if !strings.Contains(body, "e.owner_id = p_actor_entity_id") {
		t.Errorf("natural_person body missing self-access clause; got:\n%s", body)
	}
	if !strings.Contains(body, "type_is_or_descends_from(e.fundamental_type_id, 'natural_person')") {
		t.Errorf("natural_person body missing type predicate; got:\n%s", body)
	}
}

func TestGrantTableGenerator_Corporation(t *testing.T) {
	t.Parallel()
	gen := setup.NewGrantTableGenerator()
	body, err := gen.GenerateForResource("corporation")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Under the shared generic template the corporation body DOES contain the
	// own-arm text (e.owner_id = p_actor_entity_id). The "corporations have no
	// self-access" invariant now lives in the ownership data model — corporation
	// entities keep a NULL owner_id, so the own-arm matches nothing — and is
	// verified by DB/integration tests, not by SQL-text inspection here.
	if !strings.Contains(body, "type_is_or_descends_from(e.fundamental_type_id, 'corporation')") {
		t.Errorf("corporation body missing type predicate; got:\n%s", body)
	}
	// Must still include the grants CTE path.
	if !strings.Contains(body, "TargetChain") {
		t.Errorf("corporation body missing TargetChain CTE; got:\n%s", body)
	}
}

func TestGrantTableGenerator_LegalEntity(t *testing.T) {
	t.Parallel()
	gen := setup.NewGrantTableGenerator()
	body, err := gen.GenerateForResource("legal_entity")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Post-refactor legal_entity routes through the same generic body: the type
	// predicate matches both natural_person and corporation (they descend from
	// legal_entity), so the old delegation to the concrete sub-type functions is
	// gone.
	if !strings.Contains(body, "type_is_or_descends_from(e.fundamental_type_id, 'legal_entity')") {
		t.Errorf("legal_entity body missing type predicate; got:\n%s", body)
	}
	if strings.Contains(body, "accessible_natural_person_ids_for_actor") {
		t.Errorf("legal_entity body must not delegate to sub-type functions; got:\n%s", body)
	}
}

func TestGrantTableGenerator_ServiceAccount(t *testing.T) {
	t.Parallel()
	gen := setup.NewGrantTableGenerator()
	body, err := gen.GenerateForResource("service_account")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(body, "e.owner_id = p_actor_entity_id") {
		t.Errorf("service_account body missing self-access clause; got:\n%s", body)
	}
	if !strings.Contains(body, "type_is_or_descends_from(e.fundamental_type_id, 'service_account')") {
		t.Errorf("service_account body missing type predicate; got:\n%s", body)
	}
}

func TestGrantTableGenerator_InvalidSlugFormat(t *testing.T) {
	t.Parallel()
	gen := setup.NewGrantTableGenerator()
	// The error path now signals a malformed slug (SQL-injection guard on the
	// interpolated slug), not an unknown slug.
	for _, bad := range []string{"bad slug; DROP", "Tag'"} {
		if _, err := gen.GenerateForResource(bad); err == nil {
			t.Errorf("expected error for malformed slug %q; got nil", bad)
		}
	}
	// A well-formed but previously-unknown slug still generates a body: mod-core
	// has no allow-list, so a resource type it has never heard of works.
	const wellFormedSlug = "widget"
	body, err := gen.GenerateForResource(wellFormedSlug)
	if err != nil {
		t.Fatalf("well-formed slug %q: unexpected error: %v", wellFormedSlug, err)
	}
	if body == "" {
		t.Errorf("well-formed slug %q: expected non-empty body", wellFormedSlug)
	}
}

// --- InterfaceCompliance ---

func TestInterfaceCompliance(t *testing.T) {
	t.Parallel()
	var (
		_ setup.AccessFuncGenerator = setup.NewGrantTableGenerator()
		_ setup.AccessFuncGenerator = setup.PermissiveGenerator(nil)
		_ setup.AccessFuncGenerator = setup.DenyingGenerator()
	)
}
