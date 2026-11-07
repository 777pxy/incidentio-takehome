package cmd

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/777pxy/incidentio-takehome/internal/oncall"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const validSchedule = `{"users": ["alice", "bob", "charlie"], "handover_start_at": "2026-11-06T17:00:00Z", "handover_interval_days": 7}`
const validOverrides = `[{"user": "charlie", "start_at": "2026-11-09T17:00:00Z", "end_at": "2026-11-09T22:00:00Z"}]`

// writeFile writes content to a file in dir and returns its path.
func writeFile(t *testing.T, dir, name, content string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	require.NoError(t, os.WriteFile(path, []byte(content), 0o644))
	return path
}

func TestRun(t *testing.T) {
	dir := t.TempDir()
	schedule := writeFile(t, dir, "schedule.json", validSchedule)
	overrides := writeFile(t, dir, "overrides.json", validOverrides)
	badSchedule := writeFile(t, dir, "bad-schedule.json", `{"users": [], "handover_start_at": "2026-11-06T17:00:00Z", "handover_interval_days": 0}`)
	badOverrides := writeFile(t, dir, "bad-overrides.json", `[{"user": "charlie",`)
	missing := filepath.Join(dir, "missing.json")

	// argsWith returns a complete set of valid flags, with the flags in
	// changes replacing the valid values.
	argsWith := func(changes map[string]string) []string {
		flags := map[string]string{
			"schedule":  schedule,
			"overrides": overrides,
			"from":      "2026-11-06T17:00:00Z",
			"until":     "2026-11-20T17:00:00Z",
		}
		for k, v := range changes {
			flags[k] = v
		}
		var out []string
		for _, name := range []string{"schedule", "overrides", "from", "until"} {
			if v, ok := flags[name]; ok {
				out = append(out, "--"+name+"="+v)
			}
		}
		return out
	}

	tests := []struct {
		name       string
		args       []string
		wantCode   int
		wantStderr []string // substrings expected on stderr
	}{
		{
			name:     "valid input",
			args:     argsWith(nil),
			wantCode: 0,
		},
		{
			name:     "until before from is not an error",
			args:     argsWith(map[string]string{"from": "2026-11-20T17:00:00Z", "until": "2026-11-06T17:00:00Z"}),
			wantCode: 0,
		},
		{
			name:     "from before handover is not an error",
			args:     argsWith(map[string]string{"from": "2026-01-01T00:00:00Z"}),
			wantCode: 0,
		},
		{
			name:     "non-UTC offsets are accepted",
			args:     argsWith(map[string]string{"from": "2026-11-06T18:00:00+01:00"}),
			wantCode: 0,
		},
		{
			name:     "window of exactly 1 year is allowed",
			args:     argsWith(map[string]string{"from": "2026-11-06T17:00:00Z", "until": "2027-11-06T17:00:00Z"}),
			wantCode: 0,
		},
		{
			name:       "window over 1 year",
			args:       argsWith(map[string]string{"from": "2026-11-06T17:00:00Z", "until": "2027-11-06T17:00:01Z"}),
			wantCode:   1,
			wantStderr: []string{"--until (2027-11-06T17:00:01Z) must be at most 1 year after --from (2026-11-06T17:00:00Z)"},
		},
		{
			name:     "from exactly 100 years after the handover is allowed",
			args:     argsWith(map[string]string{"from": "2126-11-06T17:00:00Z", "until": "2126-11-20T17:00:00Z"}),
			wantCode: 0,
		},
		{
			name:       "from over 100 years after the handover",
			args:       argsWith(map[string]string{"from": "2126-11-06T17:00:01Z", "until": "2126-11-20T17:00:00Z"}),
			wantCode:   1,
			wantStderr: []string{"--from (2126-11-06T17:00:01Z) must be at most 100 years after handover_start_at (2026-11-06T17:00:00Z)"},
		},
		{
			name:       "no flags",
			args:       nil,
			wantCode:   1,
			wantStderr: []string{`required flag(s) "from", "overrides", "schedule", "until" not set`},
		},
		{
			name:       "unknown flag",
			args:       append(argsWith(nil), "--at=2026-11-06T17:00:00Z"),
			wantCode:   1,
			wantStderr: []string{"unknown flag: --at"},
		},
		{
			name:       "positional argument",
			args:       append(argsWith(nil), "extra"),
			wantCode:   1,
			wantStderr: []string{`unknown command "extra"`},
		},
		{
			name:       "from not RFC 3339",
			args:       argsWith(map[string]string{"from": "2026-11-06"}),
			wantCode:   1,
			wantStderr: []string{`--from "2026-11-06" must be in RFC 3339 format (YYYY-MM-DDTHH:MM:SSZ, or with an offset such as +01:00 in place of Z), e.g. 2026-11-06T17:00:00Z`},
		},
		{
			name:       "until empty",
			args:       argsWith(map[string]string{"until": ""}),
			wantCode:   1,
			wantStderr: []string{`--until "" must be in RFC 3339 format`},
		},
		{
			name:       "schedule path empty",
			args:       argsWith(map[string]string{"schedule": ""}),
			wantCode:   1,
			wantStderr: []string{"--schedule must not be empty"},
		},
		{
			name:       "schedule file missing",
			args:       argsWith(map[string]string{"schedule": missing}),
			wantCode:   1,
			wantStderr: []string{"reading --schedule file", "no such file or directory"},
		},
		{
			name:     "invalid schedule lists every problem",
			args:     argsWith(map[string]string{"schedule": badSchedule}),
			wantCode: 1,
			wantStderr: []string{
				badSchedule + ": invalid schedule:",
				"  - users: must contain at least one user",
				"  - handover_interval_days: must be at least 1",
			},
		},
		{
			name:       "malformed overrides",
			args:       argsWith(map[string]string{"overrides": badOverrides}),
			wantCode:   1,
			wantStderr: []string{badOverrides + ": malformed JSON at line 1"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			code := Run(tt.args, &stdout, &stderr)

			require.Equal(t, tt.wantCode, code, "exit code (stderr: %q)", stderr.String())
			if tt.wantCode == 0 {
				assert.Empty(t, stderr.String(), "unexpected stderr")
			} else {
				assert.Empty(t, stdout.String(), "errors must not be written to stdout")
			}
			for _, want := range tt.wantStderr {
				assert.Contains(t, stderr.String(), want)
			}
		})
	}
}

func TestParseTimeFlag(t *testing.T) {
	tests := []struct {
		name    string
		value   string
		want    string // expected time, printed in RFC 3339
		wantErr bool
	}{
		{name: "UTC stays UTC", value: "2026-11-06T17:00:00Z", want: "2026-11-06T17:00:00Z"},
		{name: "positive offset", value: "2026-11-06T18:00:00+01:00", want: "2026-11-06T17:00:00Z"},
		{name: "negative offset", value: "2026-11-06T12:00:00-05:00", want: "2026-11-06T17:00:00Z"},
		{name: "zero offset", value: "2026-11-06T17:00:00+00:00", want: "2026-11-06T17:00:00Z"},
		{name: "offset without colon", value: "2026-11-06T18:00:00+0100", wantErr: true},
		{name: "no offset or Z", value: "2026-11-06T17:00:00", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseTimeFlag("from", tt.value)
			if tt.wantErr {
				require.ErrorContains(t, err, oncall.TimeFormatHint)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, time.UTC, got.Location())
			assert.Equal(t, tt.want, got.Format(time.RFC3339))
		})
	}
}
