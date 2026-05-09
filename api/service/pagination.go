package service

const (
	DefaultPageLimit int32 = 20
	MaxPageLimit     int32 = 200
)

// Pagination is the shared limit/offset shape used across peer-module list
// endpoints. Defaults are mirrored in each module's OpenAPI parameter
// definitions (default 20, max 200); changing them here without updating the
// specs will create drift.
type Pagination struct {
	Limit  int32
	Offset int32
}

// Normalize clamps Limit to [1, MaxPageLimit] (defaulting to DefaultPageLimit
// when <= 0) and Offset to >= 0.
func (p Pagination) Normalize() (limit, offset int32) {
	switch {
	case p.Limit <= 0:
		limit = DefaultPageLimit
	case p.Limit > MaxPageLimit:
		limit = MaxPageLimit
	default:
		limit = p.Limit
	}
	if p.Offset < 0 {
		offset = 0
	} else {
		offset = p.Offset
	}
	return
}
