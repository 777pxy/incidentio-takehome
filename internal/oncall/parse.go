package oncall

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
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
	if err := decode(data, &s); err != nil {
		return Schedule{}, err
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

// ParseOverrides decodes a JSON array of overrides and validates each one on
// its own. Checks that need the schedule or the other overrides are in
// CheckOverrides.
func ParseOverrides(data []byte) ([]Override, error) {
	var overrides []Override
	if err := decode(data, &overrides); err != nil {
		return nil, err
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

// CheckOverrides sorts the overrides by start time, then rejects any that
// start before the schedule's handover or overlap another override. The brief
// says neither will happen, but if it does the output would be an invalid
// schedule, so it's safer to fail loudly. It returns the sorted overrides,
// ready for Render.
func CheckOverrides(s Schedule, overrides []Override) ([]Override, error) {
	// Sort the positions rather than the overrides themselves, so errors can
	// still point at each override's place in the file.
	order := make([]int, len(overrides))
	for i := range order {
		order[i] = i
	}
	slices.SortFunc(order, func(a, b int) int { return overrides[a].StartAt.Compare(overrides[b].StartAt) })

	var problems []string
	sorted := make([]Override, 0, len(overrides))
	latest := -1 // the override checked so far that ends latest
	for _, i := range order {
		o := overrides[i]
		if o.StartAt.Before(s.HandoverStartAt) {
			problems = append(problems, fmt.Sprintf("overrides[%d]: start_at (%s) must not be before handover_start_at (%s)",
				i, o.StartAt.Format(time.RFC3339), s.HandoverStartAt.Format(time.RFC3339)))
		}
		// Comparing with the one that ends latest, not just the previous one,
		// also catches an override inside an earlier, longer one.
		if latest >= 0 && o.StartAt.Before(overrides[latest].EndAt) {
			problems = append(problems, fmt.Sprintf("overrides[%d]: overlaps overrides[%d]", i, latest))
		}
		if latest < 0 || o.EndAt.After(overrides[latest].EndAt) {
			latest = i
		}
		sorted = append(sorted, o)
	}

	if len(problems) > 0 {
		return nil, &ValidationError{What: "overrides", Problems: problems}
	}
	return sorted, nil
}

// decode unmarshals data into v and rejects any key v has no field for.
func decode(data []byte, v any) error {
	if err := json.Unmarshal(data, v); err != nil {
		return describeJSONError(data, err)
	}
	// Unmarshal ignores unknown keys, so a misspelled key would only show up
	// later as a missing value. A second pass with DisallowUnknownFields names
	// it. The JSON is already known to be valid, so the only error left is an
	// unknown key, which Go reports as `json: unknown field "name"`.
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	if err := dec.Decode(v); err != nil {
		return fmt.Errorf("unknown key %s", strings.TrimPrefix(err.Error(), "json: unknown field "))
	}
	return nil
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
