// Package audit defines the Writer interface consumer-provided audit
// implementations must satisfy. core-module services never write to
// audit_log directly; that table lives in the consumer module.
//
// Actor identity is resolved from ctx by the implementation, typically
// via a consumer-specific context key populated by auth middleware.
//
// Deprecated: This package is superseded by the observer package which
// provides MutationObserver, a broader interface covering both in-transaction
// and post-commit observation. Migrate implementations to
// observer.MutationObserver; this package will be deleted in Phase 4.1.
package audit

import "context"

// Writer records audit events. The op string describes the operation
// (e.g. "create", "update", "delete"). The resource string names the
// domain object type (e.g. "natural_person"). targetEntityID is the
// internal entity ID of the affected record, or nil when not applicable.
// before and after are any JSON-serialisable snapshots; either may be nil.
//
// Deprecated: Use observer.MutationObserver instead. Writer will be removed
// in Phase 4.1. Existing implementations remain valid until that migration.
type Writer interface {
	Write(ctx context.Context, op, resource string, targetEntityID *int64, before, after any) error
}
