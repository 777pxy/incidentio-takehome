package oncall

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func mustTime(t *testing.T, s string) time.Time {
	t.Helper()
	parsed, err := time.Parse(time.RFC3339, s)
	require.NoError(t, err, "bad test time")
	return parsed
}

// checkErr fails the test unless err matches what the test case expects:
// no error, an error containing wantErr, or a ValidationError with exactly
// wantProblems.
func checkErr(t *testing.T, err error, wantErr string, wantProblems []string) {
	t.Helper()
	switch {
	case wantProblems != nil:
		var vErr *ValidationError
		require.ErrorAs(t, err, &vErr)
		assert.Equal(t, wantProblems, vErr.Problems)
	case wantErr != "":
		require.ErrorContains(t, err, wantErr)
	default:
		require.NoError(t, err)
	}
}

func TestParseSchedule(t *testing.T) {
	tests := []struct {
		name         string
		json         string
		want         Schedule
		wantErr      string   // substring of a decoding error
		wantProblems []string // exact validation problems
	}{
		{
			name: "valid",
			json: `{"users": ["alice", "bob"], "handover_start_at": "2026-11-06T17:00:00Z", "handover_interval_days": 7}`,
			want: Schedule{
				Users:                []string{"alice", "bob"},
				HandoverStartAt:      mustTime(t, "2026-11-06T17:00:00Z"),
				HandoverIntervalDays: 7,
			},
		},
		{
			name: "same user more than once is kept as the same person",
			json: `{"users": ["alice", "bob", "alice"], "handover_start_at": "2026-11-06T17:00:00Z", "handover_interval_days": 1}`,
			want: Schedule{
				Users:                []string{"alice", "bob", "alice"},
				HandoverStartAt:      mustTime(t, "2026-11-06T17:00:00Z"),
				HandoverIntervalDays: 1,
			},
		},
		{
			name: "handover_start_at is converted to UTC",
			json: `{"users": ["alice"], "handover_start_at": "2026-11-06T18:00:00+01:00", "handover_interval_days": 7}`,
			want: Schedule{
				Users:                []string{"alice"},
				HandoverStartAt:      mustTime(t, "2026-11-06T17:00:00Z"),
				HandoverIntervalDays: 7,
			},
		},
		{
			name:    "empty file",
			json:    ``,
			wantErr: "malformed JSON at line 1, column 1",
		},
		{
			name:    "syntax error reports line and column",
			json:    "{\n  \"users\": [\"alice\",]\n}",
			wantErr: "malformed JSON at line 2, column 21",
		},
		{
			name:    "trailing data",
			json:    `{} {}`,
			wantErr: "malformed JSON",
		},
		{
			name:    "array instead of object",
			json:    `[]`,
			wantErr: "wrong type at line 1",
		},
		{
			name:    "users is not a list",
			json:    `{"users": "alice"}`,
			wantErr: "wrong type",
		},
		{
			name:    "fractional interval",
			json:    `{"users": ["alice"], "handover_start_at": "2026-11-06T17:00:00Z", "handover_interval_days": 1.5}`,
			wantErr: "wrong type",
		},
		{
			name:    "misspelled key",
			json:    `{"users": ["alice"], "handover_start_at": "2026-11-06T17:00:00Z", "handover_intervl_days": 7}`,
			wantErr: `unknown key "handover_intervl_days"`,
		},
		{
			name:    "handover_start_at not RFC 3339",
			json:    `{"users": ["alice"], "handover_start_at": "2026-11-06", "handover_interval_days": 7}`,
			wantErr: `timestamp "2026-11-06" must be in RFC 3339 format (YYYY-MM-DDTHH:MM:SSZ, or with an offset such as +01:00 in place of Z), e.g. 2026-11-06T17:00:00Z`,
		},
		{
			name: "null reports every missing field",
			json: `null`,
			wantProblems: []string{
				"users: must contain at least one user",
				"handover_start_at: is missing",
				"handover_interval_days: must be at least 1",
			},
		},
		{
			name:         "empty users",
			json:         `{"users": [], "handover_start_at": "2026-11-06T17:00:00Z", "handover_interval_days": 7}`,
			wantProblems: []string{"users: must contain at least one user"},
		},
		{
			name:         "blank user",
			json:         `{"users": ["alice", " "], "handover_start_at": "2026-11-06T17:00:00Z", "handover_interval_days": 7}`,
			wantProblems: []string{"users[1]: must not be empty"},
		},
		{
			name:         "negative interval",
			json:         `{"users": ["alice"], "handover_start_at": "2026-11-06T17:00:00Z", "handover_interval_days": -7}`,
			wantProblems: []string{"handover_interval_days: must be at least 1"},
		},
		{
			name: "interval of 1 year is allowed",
			json: `{"users": ["alice"], "handover_start_at": "2026-11-06T17:00:00Z", "handover_interval_days": 365}`,
			want: Schedule{
				Users:                []string{"alice"},
				HandoverStartAt:      mustTime(t, "2026-11-06T17:00:00Z"),
				HandoverIntervalDays: 365,
			},
		},
		{
			name:         "interval over 1 year",
			json:         `{"users": ["alice"], "handover_start_at": "2026-11-06T17:00:00Z", "handover_interval_days": 366}`,
			wantProblems: []string{"handover_interval_days: must be at most 365"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseSchedule([]byte(tt.json))
			checkErr(t, err, tt.wantErr, tt.wantProblems)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestParseOverrides(t *testing.T) {
	tests := []struct {
		name         string
		json         string
		want         []Override
		wantErr      string
		wantProblems []string
	}{
		{
			name: "valid",
			json: `[{"user": "charlie", "start_at": "2026-11-09T17:00:00Z", "end_at": "2026-11-09T22:00:00Z"}]`,
			want: []Override{{
				User:    "charlie",
				StartAt: mustTime(t, "2026-11-09T17:00:00Z"),
				EndAt:   mustTime(t, "2026-11-09T22:00:00Z"),
			}},
		},
		{
			name: "times are converted to UTC",
			json: `[{"user": "charlie", "start_at": "2026-11-09T12:00:00-05:00", "end_at": "2026-11-09T23:00:00+01:00"}]`,
			want: []Override{{
				User:    "charlie",
				StartAt: mustTime(t, "2026-11-09T17:00:00Z"),
				EndAt:   mustTime(t, "2026-11-09T22:00:00Z"),
			}},
		},
		{
			name: "no overrides",
			json: `[]`,
			want: []Override{},
		},
		{
			name: "unsorted overrides are kept in file order",
			json: `[
				{"user": "bob", "start_at": "2026-11-10T00:00:00Z", "end_at": "2026-11-10T01:00:00Z"},
				{"user": "alice", "start_at": "2026-11-09T00:00:00Z", "end_at": "2026-11-09T01:00:00Z"}
			]`,
			want: []Override{
				{User: "bob", StartAt: mustTime(t, "2026-11-10T00:00:00Z"), EndAt: mustTime(t, "2026-11-10T01:00:00Z")},
				{User: "alice", StartAt: mustTime(t, "2026-11-09T00:00:00Z"), EndAt: mustTime(t, "2026-11-09T01:00:00Z")},
			},
		},
		{
			name:    "object instead of array",
			json:    `{"user": "charlie"}`,
			wantErr: "wrong type",
		},
		{
			name:    "malformed",
			json:    `[{"user": "charlie"`,
			wantErr: "malformed JSON",
		},
		{
			name:    "misspelled key",
			json:    `[{"user": "charlie", "start": "2026-11-09T17:00:00Z", "end_at": "2026-11-09T22:00:00Z"}]`,
			wantErr: `unknown key "start"`,
		},
		{
			name:    "bad timestamp",
			json:    `[{"user": "charlie", "start_at": "tomorrow", "end_at": "2026-11-09T22:00:00Z"}]`,
			wantErr: `timestamp "tomorrow" must be in RFC 3339 format`,
		},
		{
			name: "empty object reports every missing field",
			json: `[{}]`,
			wantProblems: []string{
				"overrides[0].user: must not be empty",
				"overrides[0].start_at: is missing",
				"overrides[0].end_at: is missing",
			},
		},
		{
			name:         "end before start",
			json:         `[{"user": "charlie", "start_at": "2026-11-09T22:00:00Z", "end_at": "2026-11-09T17:00:00Z"}]`,
			wantProblems: []string{"overrides[0]: end_at (2026-11-09T17:00:00Z) must be after start_at (2026-11-09T22:00:00Z)"},
		},
		{
			name:         "zero length",
			json:         `[{"user": "charlie", "start_at": "2026-11-09T17:00:00Z", "end_at": "2026-11-09T17:00:00Z"}]`,
			wantProblems: []string{"overrides[0]: end_at (2026-11-09T17:00:00Z) must be after start_at (2026-11-09T17:00:00Z)"},
		},
		{
			name: "problems are reported for every override",
			json: `[
				{"user": "", "start_at": "2026-11-09T17:00:00Z", "end_at": "2026-11-09T22:00:00Z"},
				{"user": "alice", "start_at": "2026-11-10T17:00:00Z", "end_at": "2026-11-10T22:00:00Z"},
				{"user": "bob", "start_at": "2026-11-11T17:00:00Z"}
			]`,
			wantProblems: []string{
				"overrides[0].user: must not be empty",
				"overrides[2].end_at: is missing",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseOverrides([]byte(tt.json))
			checkErr(t, err, tt.wantErr, tt.wantProblems)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestCheckOverrides(t *testing.T) {
	schedule := Schedule{
		Users:                []string{"alice", "bob"},
		HandoverStartAt:      mustTime(t, "2026-11-06T17:00:00Z"),
		HandoverIntervalDays: 7,
	}
	override := func(start, end string) Override {
		return Override{User: "charlie", StartAt: mustTime(t, start), EndAt: mustTime(t, end)}
	}

	tests := []struct {
		name         string
		overrides    []Override
		want         []Override // sorted by start time
		wantProblems []string   // nil means no error
	}{
		{
			name: "no overrides",
			want: []Override{},
		},
		{
			name: "overrides are returned sorted",
			overrides: []Override{
				override("2026-11-10T00:00:00Z", "2026-11-10T06:00:00Z"),
				override("2026-11-08T00:00:00Z", "2026-11-08T06:00:00Z"),
			},
			want: []Override{
				override("2026-11-08T00:00:00Z", "2026-11-08T06:00:00Z"),
				override("2026-11-10T00:00:00Z", "2026-11-10T06:00:00Z"),
			},
		},
		{
			name:      "starting exactly at the handover is allowed",
			overrides: []Override{override("2026-11-06T17:00:00Z", "2026-11-06T18:00:00Z")},
			want:      []Override{override("2026-11-06T17:00:00Z", "2026-11-06T18:00:00Z")},
		},
		{
			name: "touching overrides are allowed",
			overrides: []Override{
				override("2026-11-09T17:00:00Z", "2026-11-09T18:00:00Z"),
				override("2026-11-09T18:00:00Z", "2026-11-09T19:00:00Z"),
			},
			want: []Override{
				override("2026-11-09T17:00:00Z", "2026-11-09T18:00:00Z"),
				override("2026-11-09T18:00:00Z", "2026-11-09T19:00:00Z"),
			},
		},
		{
			name:         "before the handover",
			overrides:    []Override{override("2026-11-06T16:00:00Z", "2026-11-06T18:00:00Z")},
			wantProblems: []string{"overrides[0]: start_at (2026-11-06T16:00:00Z) must not be before handover_start_at (2026-11-06T17:00:00Z)"},
		},
		{
			name: "overlapping, in reverse order",
			overrides: []Override{
				override("2026-11-09T18:00:00Z", "2026-11-09T20:00:00Z"),
				override("2026-11-09T17:00:00Z", "2026-11-09T19:00:00Z"),
			},
			wantProblems: []string{"overrides[0]: overlaps overrides[1]"},
		},
		{
			name: "inside an earlier, longer override",
			overrides: []Override{
				override("2026-11-09T00:00:00Z", "2026-11-10T00:00:00Z"),
				override("2026-11-09T01:00:00Z", "2026-11-09T02:00:00Z"),
				override("2026-11-09T03:00:00Z", "2026-11-09T04:00:00Z"),
			},
			wantProblems: []string{
				"overrides[1]: overlaps overrides[0]",
				"overrides[2]: overlaps overrides[0]",
			},
		},
		{
			name: "every problem is reported",
			overrides: []Override{
				override("2026-11-06T16:00:00Z", "2026-11-06T18:00:00Z"),
				override("2026-11-06T17:30:00Z", "2026-11-06T19:00:00Z"),
			},
			wantProblems: []string{
				"overrides[0]: start_at (2026-11-06T16:00:00Z) must not be before handover_start_at (2026-11-06T17:00:00Z)",
				"overrides[1]: overlaps overrides[0]",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := CheckOverrides(schedule, tt.overrides)
			checkErr(t, err, "", tt.wantProblems)
			assert.Equal(t, tt.want, got)
		})
	}
}
