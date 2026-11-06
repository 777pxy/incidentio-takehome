package oncall

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGenerateShifts(t *testing.T) {
	// The README's example: alice, bob, charlie rotating weekly from
	// 5pm, Friday 6th November 2026.
	schedule := Schedule{
		Users:                []string{"alice", "bob", "charlie"},
		HandoverStartAt:      mustTime(t, "2026-11-06T17:00:00Z"),
		HandoverIntervalDays: 7,
	}

	entry := func(user, start, end string) Entry {
		return Entry{User: user, StartAt: mustTime(t, start), EndAt: mustTime(t, end)}
	}

	tests := []struct {
		name  string
		from  string
		until string
		want  []Entry
	}{
		{
			name:  "window starting mid-shift",
			from:  "2026-11-08T12:00:00Z",
			until: "2026-11-20T17:00:00Z",
			want: []Entry{
				entry("alice", "2026-11-08T12:00:00Z", "2026-11-13T17:00:00Z"),
				entry("bob", "2026-11-13T17:00:00Z", "2026-11-20T17:00:00Z"),
			},
		},
		{
			name:  "wraps back to the first user",
			from:  "2026-11-06T17:00:00Z",
			until: "2026-11-30T17:00:00Z",
			want: []Entry{
				entry("alice", "2026-11-06T17:00:00Z", "2026-11-13T17:00:00Z"),
				entry("bob", "2026-11-13T17:00:00Z", "2026-11-20T17:00:00Z"),
				entry("charlie", "2026-11-20T17:00:00Z", "2026-11-27T17:00:00Z"),
				entry("alice", "2026-11-27T17:00:00Z", "2026-11-30T17:00:00Z"),
			},
		},
		{
			name:  "from before the anchor starts at the anchor",
			from:  "2026-11-01T00:00:00Z",
			until: "2026-11-16T00:00:00Z",
			want: []Entry{
				entry("alice", "2026-11-06T17:00:00Z", "2026-11-13T17:00:00Z"),
				entry("bob", "2026-11-13T17:00:00Z", "2026-11-16T00:00:00Z"),
			},
		},
		{
			name:  "until at the anchor",
			from:  "2026-11-01T00:00:00Z",
			until: "2026-11-06T17:00:00Z",
			want:  []Entry{},
		},
		{
			name:  "until before the anchor",
			from:  "2026-11-01T00:00:00Z",
			until: "2026-11-03T00:00:00Z",
			want:  []Entry{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := GenerateShifts(schedule, mustTime(t, tt.from), mustTime(t, tt.until))
			assert.Equal(t, tt.want, got)
		})
	}
}
