package oncall

import "time"

// GenerateShifts returns the rotation's shifts between from and until, before
// any overrides are applied. The first shift starts at from and the last is
// cut off at until.
func GenerateShifts(s Schedule, from, until time.Time) []Entry {
	shifts := []Entry{}

	// Nobody is on call before the first handover.
	if from.Before(s.HandoverStartAt) {
		from = s.HandoverStartAt
	}
	// Covers both until <= handover_start_at (from was just clamped to it)
	// and until <= from.
	if !until.After(from) {
		return shifts
	}

	period := time.Duration(s.HandoverIntervalDays) * 24 * time.Hour

	// How many full shifts have happened between the anchor and from.
	// from >= the anchor after clamping, so integer division is a floor.
	shiftNo := int(from.Sub(s.HandoverStartAt) / period)

	firstShiftStart := s.HandoverStartAt.Add(time.Duration(shiftNo) * period)
	start := from // the first shift is cut to start at from
	end := firstShiftStart.Add(period)

	for start.Before(until) {
		if end.After(until) {
			end = until
		}
		shifts = append(shifts, Entry{
			User:    s.Users[shiftNo%len(s.Users)],
			StartAt: start,
			EndAt:   end,
		})
		start = end
		end = start.Add(period)
		shiftNo++
	}
	return shifts
}
