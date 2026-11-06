package oncall

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
)

// maxIntervalDays caps shifts at 1 year. Nobody should be on call for more
// than a month, let alone a year, so anything longer is an input mistake.
const maxIntervalDays = 365

// TimeFormatHint describes the timestamp format every input must use. Any UTC
// offset is accepted, and parsed times are converted to UTC.
const TimeFormatHint = "RFC 3339 format (YYYY-MM-DDTHH:MM:SSZ, or with an offset such as +01:00 in place of Z), e.g. 2026-11-06T17:00:00Z"

// ValidationError lists every problem found in input that parsed as JSON but
// doesn't contain the right information.
type ValidationError struct {
	What     string   // what was being validated, e.g. "schedule"
	Problems []string // one human-readable problem per entry
}

func (e *ValidationError) Error() string {
	var b strings.Builder
	fmt.Fprintf(&b, "invalid %s:", e.What)
	for _, p := range e.Problems {
		b.WriteString("\n  - ")
		b.WriteString(p)
	}
	return b.String()
}

// ParseSchedule decodes a schedule from JSON and validates it.
func ParseSchedule(data []byte) (Schedule, error) {
	var s Schedule
	if err := json.Unmarshal(data, &s); err != nil {
		return Schedule{}, describeJSONError(data, err)
	}

	var problems []string
	if len(s.Users) == 0 {
		problems = append(problems, "users: must contain at least one user")
	}
	for i, u := range s.Users {
		if strings.TrimSpace(u) == "" {
			problems = append(problems, fmt.Sprintf("users[%d]: must not be empty", i))
		}
	}
	if s.HandoverStartAt.IsZero() {
		problems = append(problems, "handover_start_at: is missing")
	}
	switch {
	case s.HandoverIntervalDays < 1:
		problems = append(problems, "handover_interval_days: must be at least 1")
	case s.HandoverIntervalDays > maxIntervalDays:
		problems = append(problems, fmt.Sprintf("handover_interval_days: must be at most %d", maxIntervalDays))
	}

	if len(problems) > 0 {
		return Schedule{}, &ValidationError{What: "schedule", Problems: problems}
	}
	// Work in UTC so times print consistently and compare equal with ==.
	s.HandoverStartAt = s.HandoverStartAt.UTC()
	return s, nil
}

// ParseOverrides decodes a JSON array of overrides and validates each one.
// It doesn't check for overlaps or overrides before the schedule starts, as
// the brief guarantees neither will happen.
func ParseOverrides(data []byte) ([]Override, error) {
	var overrides []Override
	if err := json.Unmarshal(data, &overrides); err != nil {
		return nil, describeJSONError(data, err)
	}

	var problems []string
	for i, o := range overrides {
		at := fmt.Sprintf("overrides[%d]", i)
		if strings.TrimSpace(o.User) == "" {
			problems = append(problems, at+".user: must not be empty")
		}
		if o.StartAt.IsZero() {
			problems = append(problems, at+".start_at: is missing")
		}
		if o.EndAt.IsZero() {
			problems = append(problems, at+".end_at: is missing")
		}
		if !o.StartAt.IsZero() && !o.EndAt.IsZero() && !o.EndAt.After(o.StartAt) {
			problems = append(problems, fmt.Sprintf("%s: end_at (%s) must be after start_at (%s)",
				at, o.EndAt.Format(time.RFC3339), o.StartAt.Format(time.RFC3339)))
		}
	}

	if len(problems) > 0 {
		return nil, &ValidationError{What: "overrides", Problems: problems}
	}
	for i := range overrides {
		overrides[i].StartAt = overrides[i].StartAt.UTC()
		overrides[i].EndAt = overrides[i].EndAt.UTC()
	}
	return overrides, nil
}

// describeJSONError turns a decoding error into a message that says where in
// the file the problem is.
func describeJSONError(data []byte, err error) error {
	var syntaxErr *json.SyntaxError
	var typeErr *json.UnmarshalTypeError
	var timeErr *time.ParseError
	switch {
	case errors.As(err, &syntaxErr):
		line, col := position(data, syntaxErr.Offset)
		return fmt.Errorf("malformed JSON at line %d, column %d: %w", line, col, err)
	case errors.As(err, &typeErr):
		line, col := position(data, typeErr.Offset)
		return fmt.Errorf("wrong type at line %d, column %d: %w", line, col, err)
	case errors.As(err, &timeErr):
		return fmt.Errorf("timestamp %q must be in %s", timeErr.Value, TimeFormatHint)
	default:
		return fmt.Errorf("could not decode JSON: %w", err)
	}
}

// position converts a byte offset into a 1-based line and column.
func position(data []byte, offset int64) (line, col int) {
	before := data[:min(int(offset), len(data))]
	line = 1 + bytes.Count(before, []byte("\n"))
	col = len(before) - bytes.LastIndexByte(before, '\n') - 1
	return line, max(col, 1)
}
