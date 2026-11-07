package oncall

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRender(t *testing.T) {
	// The brief's schedule: alice, bob, charlie rotating weekly from
	// 5pm, Friday 6th November 2026. Shifts hand over at 17:00 on the 6th,
	// 13th, 20th and 27th.
	schedule := Schedule{
		Users:                []string{"alice", "bob", "charlie"},
		HandoverStartAt:      mustTime(t, "2026-11-06T17:00:00Z"),
		HandoverIntervalDays: 7,
	}

	entry := func(user, start, end string) Entry {
		return Entry{User: user, StartAt: mustTime(t, start), EndAt: mustTime(t, end)}
	}
	override := func(user, start, end string) Override {
		return Override{User: user, StartAt: mustTime(t, start), EndAt: mustTime(t, end)}
	}

	tests := []struct {
		name      string
		overrides []Override
		from      string
		until     string
		want      []Entry
	}{
		{
			name: "brief's example",
			overrides: []Override{
				override("charlie", "2026-11-09T17:00:00Z", "2026-11-09T22:00:00Z"),
			},
			from:  "2026-11-06T17:00:00Z",
			until: "2026-11-20T17:00:00Z",
			want: []Entry{
				entry("alice", "2026-11-06T17:00:00Z", "2026-11-09T17:00:00Z"),
				entry("charlie", "2026-11-09T17:00:00Z", "2026-11-09T22:00:00Z"),
				entry("alice", "2026-11-09T22:00:00Z", "2026-11-13T17:00:00Z"),
				entry("bob", "2026-11-13T17:00:00Z", "2026-11-20T17:00:00Z"),
			},
		},
		{
			name: "override spanning a handover",
			overrides: []Override{
				override("dave", "2026-11-12T17:00:00Z", "2026-11-14T17:00:00Z"),
			},
			from:  "2026-11-06T17:00:00Z",
			until: "2026-11-20T17:00:00Z",
			want: []Entry{
				entry("alice", "2026-11-06T17:00:00Z", "2026-11-12T17:00:00Z"),
				entry("dave", "2026-11-12T17:00:00Z", "2026-11-14T17:00:00Z"),
				entry("bob", "2026-11-14T17:00:00Z", "2026-11-20T17:00:00Z"),
			},
		},
		{
			name: "override covering a whole shift",
			overrides: []Override{
				override("dave", "2026-11-13T17:00:00Z", "2026-11-20T17:00:00Z"),
			},
			from:  "2026-11-06T17:00:00Z",
			until: "2026-11-27T17:00:00Z",
			want: []Entry{
				entry("alice", "2026-11-06T17:00:00Z", "2026-11-13T17:00:00Z"),
				entry("dave", "2026-11-13T17:00:00Z", "2026-11-20T17:00:00Z"),
				entry("charlie", "2026-11-20T17:00:00Z", "2026-11-27T17:00:00Z"),
			},
		},
		{
			name: "two overrides inside one shift",
			overrides: []Override{
				override("dave", "2026-11-08T00:00:00Z", "2026-11-08T06:00:00Z"),
				override("erin", "2026-11-10T00:00:00Z", "2026-11-10T06:00:00Z"),
			},
			from:  "2026-11-06T17:00:00Z",
			until: "2026-11-20T17:00:00Z",
			want: []Entry{
				entry("alice", "2026-11-06T17:00:00Z", "2026-11-08T00:00:00Z"),
				entry("dave", "2026-11-08T00:00:00Z", "2026-11-08T06:00:00Z"),
				entry("alice", "2026-11-08T06:00:00Z", "2026-11-10T00:00:00Z"),
				entry("erin", "2026-11-10T00:00:00Z", "2026-11-10T06:00:00Z"),
				entry("alice", "2026-11-10T06:00:00Z", "2026-11-13T17:00:00Z"),
				entry("bob", "2026-11-13T17:00:00Z", "2026-11-20T17:00:00Z"),
			},
		},
		{
			name: "many overrides in one shift",
			overrides: []Override{
				override("dave", "2026-11-07T00:00:00Z", "2026-11-07T06:00:00Z"),
				override("erin", "2026-11-07T06:00:00Z", "2026-11-07T12:00:00Z"), // straight after dave
				override("frank", "2026-11-08T00:00:00Z", "2026-11-08T06:00:00Z"),
				override("dave", "2026-11-12T00:00:00Z", "2026-11-13T17:00:00Z"), // ends at the handover
			},
			from:  "2026-11-06T17:00:00Z",
			until: "2026-11-20T17:00:00Z",
			want: []Entry{
				entry("alice", "2026-11-06T17:00:00Z", "2026-11-07T00:00:00Z"),
				entry("dave", "2026-11-07T00:00:00Z", "2026-11-07T06:00:00Z"),
				entry("erin", "2026-11-07T06:00:00Z", "2026-11-07T12:00:00Z"),
				entry("alice", "2026-11-07T12:00:00Z", "2026-11-08T00:00:00Z"),
				entry("frank", "2026-11-08T00:00:00Z", "2026-11-08T06:00:00Z"),
				entry("alice", "2026-11-08T06:00:00Z", "2026-11-12T00:00:00Z"),
				entry("dave", "2026-11-12T00:00:00Z", "2026-11-13T17:00:00Z"),
				entry("bob", "2026-11-13T17:00:00Z", "2026-11-20T17:00:00Z"),
			},
		},
		{
			name: "override spanning several shifts is output once",
			overrides: []Override{
				override("dave", "2026-11-10T00:00:00Z", "2026-11-25T00:00:00Z"),
			},
			from:  "2026-11-06T17:00:00Z",
			until: "2026-11-27T17:00:00Z",
			want: []Entry{
				entry("alice", "2026-11-06T17:00:00Z", "2026-11-10T00:00:00Z"),
				entry("dave", "2026-11-10T00:00:00Z", "2026-11-25T00:00:00Z"),
				entry("charlie", "2026-11-25T00:00:00Z", "2026-11-27T17:00:00Z"),
			},
		},
		{
			name: "override by the user already on call merges into their shift",
			overrides: []Override{
				override("alice", "2026-11-08T00:00:00Z", "2026-11-09T00:00:00Z"),
			},
			from:  "2026-11-06T17:00:00Z",
			until: "2026-11-20T17:00:00Z",
			want: []Entry{
				entry("alice", "2026-11-06T17:00:00Z", "2026-11-13T17:00:00Z"),
				entry("bob", "2026-11-13T17:00:00Z", "2026-11-20T17:00:00Z"),
			},
		},
		{
			name: "back-to-back overrides by the same user merge",
			overrides: []Override{
				override("dave", "2026-11-08T00:00:00Z", "2026-11-08T06:00:00Z"),
				override("dave", "2026-11-08T06:00:00Z", "2026-11-08T12:00:00Z"),
			},
			from:  "2026-11-06T17:00:00Z",
			until: "2026-11-20T17:00:00Z",
			want: []Entry{
				entry("alice", "2026-11-06T17:00:00Z", "2026-11-08T00:00:00Z"),
				entry("dave", "2026-11-08T00:00:00Z", "2026-11-08T12:00:00Z"),
				entry("alice", "2026-11-08T12:00:00Z", "2026-11-13T17:00:00Z"),
				entry("bob", "2026-11-13T17:00:00Z", "2026-11-20T17:00:00Z"),
			},
		},
		{
			name: "override ending at the same user's next shift merges into it",
			overrides: []Override{
				override("bob", "2026-11-12T00:00:00Z", "2026-11-13T17:00:00Z"),
			},
			from:  "2026-11-06T17:00:00Z",
			until: "2026-11-20T17:00:00Z",
			want: []Entry{
				entry("alice", "2026-11-06T17:00:00Z", "2026-11-12T00:00:00Z"),
				entry("bob", "2026-11-12T00:00:00Z", "2026-11-20T17:00:00Z"),
			},
		},
		{
			name: "override starting before from is cut to from",
			overrides: []Override{
				override("dave", "2026-11-07T00:00:00Z", "2026-11-09T00:00:00Z"),
			},
			from:  "2026-11-08T00:00:00Z",
			until: "2026-11-20T17:00:00Z",
			want: []Entry{
				entry("dave", "2026-11-08T00:00:00Z", "2026-11-09T00:00:00Z"),
				entry("alice", "2026-11-09T00:00:00Z", "2026-11-13T17:00:00Z"),
				entry("bob", "2026-11-13T17:00:00Z", "2026-11-20T17:00:00Z"),
			},
		},
		{
			name: "override entirely before from is dropped",
			overrides: []Override{
				override("dave", "2026-11-07T00:00:00Z", "2026-11-07T06:00:00Z"),
			},
			from:  "2026-11-08T00:00:00Z",
			until: "2026-11-20T17:00:00Z",
			want: []Entry{
				entry("alice", "2026-11-08T00:00:00Z", "2026-11-13T17:00:00Z"),
				entry("bob", "2026-11-13T17:00:00Z", "2026-11-20T17:00:00Z"),
			},
		},
		{
			name: "override ending after until is cut to until",
			overrides: []Override{
				override("dave", "2026-11-19T00:00:00Z", "2026-11-22T00:00:00Z"),
			},
			from:  "2026-11-06T17:00:00Z",
			until: "2026-11-20T17:00:00Z",
			want: []Entry{
				entry("alice", "2026-11-06T17:00:00Z", "2026-11-13T17:00:00Z"),
				entry("bob", "2026-11-13T17:00:00Z", "2026-11-19T00:00:00Z"),
				entry("dave", "2026-11-19T00:00:00Z", "2026-11-20T17:00:00Z"),
			},
		},
		{
			name: "override entirely after until is dropped",
			overrides: []Override{
				override("dave", "2026-11-25T00:00:00Z", "2026-11-26T00:00:00Z"),
			},
			from:  "2026-11-06T17:00:00Z",
			until: "2026-11-20T17:00:00Z",
			want: []Entry{
				entry("alice", "2026-11-06T17:00:00Z", "2026-11-13T17:00:00Z"),
				entry("bob", "2026-11-13T17:00:00Z", "2026-11-20T17:00:00Z"),
			},
		},
		{
			name: "overrides out of order are sorted",
			overrides: []Override{
				override("erin", "2026-11-10T00:00:00Z", "2026-11-10T06:00:00Z"),
				override("dave", "2026-11-08T00:00:00Z", "2026-11-08T06:00:00Z"),
			},
			from:  "2026-11-06T17:00:00Z",
			until: "2026-11-13T17:00:00Z",
			want: []Entry{
				entry("alice", "2026-11-06T17:00:00Z", "2026-11-08T00:00:00Z"),
				entry("dave", "2026-11-08T00:00:00Z", "2026-11-08T06:00:00Z"),
				entry("alice", "2026-11-08T06:00:00Z", "2026-11-10T00:00:00Z"),
				entry("erin", "2026-11-10T00:00:00Z", "2026-11-10T06:00:00Z"),
				entry("alice", "2026-11-10T06:00:00Z", "2026-11-13T17:00:00Z"),
			},
		},
		{
			name:      "no overrides gives the generated shifts",
			overrides: []Override{},
			from:      "2026-11-06T17:00:00Z",
			until:     "2026-11-20T17:00:00Z",
			want: []Entry{
				entry("alice", "2026-11-06T17:00:00Z", "2026-11-13T17:00:00Z"),
				entry("bob", "2026-11-13T17:00:00Z", "2026-11-20T17:00:00Z"),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Render(schedule, tt.overrides, mustTime(t, tt.from), mustTime(t, tt.until))
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestRenderSingleUser(t *testing.T) {
	// With one user, every shift is theirs, so the touching shifts merge
	// into one entry covering the whole window.
	schedule := Schedule{
		Users:                []string{"alice"},
		HandoverStartAt:      mustTime(t, "2026-11-06T17:00:00Z"),
		HandoverIntervalDays: 7,
	}

	got := Render(schedule, nil, mustTime(t, "2026-11-06T17:00:00Z"), mustTime(t, "2026-11-27T17:00:00Z"))

	want := []Entry{{
		User:    "alice",
		StartAt: mustTime(t, "2026-11-06T17:00:00Z"),
		EndAt:   mustTime(t, "2026-11-27T17:00:00Z"),
	}}
	assert.Equal(t, want, got)
}
