package service_test

import (
	"testing"

	"github.com/moduleforge/core-api/service"
)

func TestPagination_Normalize(t *testing.T) {
	tests := []struct {
		name       string
		p          service.Pagination
		wantLimit  int32
		wantOffset int32
	}{
		{
			name:       "zero values yield defaults",
			p:          service.Pagination{},
			wantLimit:  service.DefaultPageLimit,
			wantOffset: 0,
		},
		{
			name:       "negative limit yields default",
			p:          service.Pagination{Limit: -5},
			wantLimit:  service.DefaultPageLimit,
			wantOffset: 0,
		},
		{
			name:       "limit above max is clamped to max",
			p:          service.Pagination{Limit: 500},
			wantLimit:  service.MaxPageLimit,
			wantOffset: 0,
		},
		{
			name:       "limit exactly at max passes through",
			p:          service.Pagination{Limit: service.MaxPageLimit},
			wantLimit:  service.MaxPageLimit,
			wantOffset: 0,
		},
		{
			name:       "mid-range limit passes through unchanged",
			p:          service.Pagination{Limit: 50, Offset: 100},
			wantLimit:  50,
			wantOffset: 100,
		},
		{
			name:       "negative offset clamped to zero",
			p:          service.Pagination{Limit: 10, Offset: -1},
			wantLimit:  10,
			wantOffset: 0,
		},
		{
			name:       "limit 1 is the minimum valid positive value",
			p:          service.Pagination{Limit: 1},
			wantLimit:  1,
			wantOffset: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotLimit, gotOffset := tt.p.Normalize()
			if gotLimit != tt.wantLimit {
				t.Errorf("limit: got %d, want %d", gotLimit, tt.wantLimit)
			}
			if gotOffset != tt.wantOffset {
				t.Errorf("offset: got %d, want %d", gotOffset, tt.wantOffset)
			}
		})
	}
}
