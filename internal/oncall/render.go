package oncall

import (
	"slices"
	"time"
)

// Render returns the final schedule between from and until: the rotation's
// shifts with the overrides applied, cut to the window, and with touching
// entries for the same user merged.
func Render(s Schedule, overrides []Override, from, until time.Time) []Entry {
	shifts := GenerateShifts(s, from, until)
	entries := applyOverrides(shifts, overrides)
	entries = truncate(entries, from, until)
	return mergeTouching(entries)
}

// applyOverrides walks the shifts and the overrides together, both in time
// order. Each override cuts the shifts it overlaps down to the parts outside
// it, and is output once in their place.
func applyOverrides(shifts []Entry, overrides []Override) []Entry {
	sorted := slices.Clone(overrides)
	slices.SortFunc(sorted, func(a, b Override) int { return a.StartAt.Compare(b.StartAt) })

	// Copied so that carrying a right part forward doesn't change the caller's
	// slice.
	shifts = slices.Clone(shifts)

	out := []Entry{}
	i := 0 // the next shift to look at
	for _, o := range sorted {
		override := Entry{User: o.User, StartAt: o.StartAt, EndAt: o.EndAt}
		overrideOutput := false

		// 1. Output every shift that ends at or before the override starts.
		for i < len(shifts) && !shifts[i].EndAt.After(o.StartAt) {
			out = append(out, shifts[i])
			i++
		}

		// 2. Every shift that starts before the override ends now overlaps it,
		// so keep only the parts of it outside the override.
		for i < len(shifts) && shifts[i].StartAt.Before(o.EndAt) {
			shift := shifts[i]
			left := Entry{User: shift.User, StartAt: shift.StartAt, EndAt: o.StartAt}
			right := Entry{User: shift.User, StartAt: o.EndAt, EndAt: shift.EndAt}

			// 3. Output the left part.
			if hasLength(left) {
				out = append(out, left)
			}
			// 4. Output the override once, at the first shift it overlaps.
			if !overrideOutput {
				out = append(out, override)
				overrideOutput = true
			}
			// 5. Keep the right part as the current shift, as the next override
			// may cut into it too.
			if hasLength(right) {
				shifts[i] = right
				break
			}
			i++
		}
	}

	// Output the shifts after the last override.
	return append(out, shifts[i:]...)
}

// truncate cuts every entry to the window, dropping any left with no length.
func truncate(entries []Entry, from, until time.Time) []Entry {
	out := []Entry{}
	for _, e := range entries {
		e.StartAt = latest(e.StartAt, from)
		e.EndAt = earliest(e.EndAt, until)
		if hasLength(e) {
			out = append(out, e)
		}
	}
	return out
}

// mergeTouching joins entries for the same user where one ends exactly when
// the next starts.
func mergeTouching(entries []Entry) []Entry {
	out := []Entry{}
	for _, e := range entries {
		if n := len(out); n > 0 && out[n-1].User == e.User && out[n-1].EndAt.Equal(e.StartAt) {
			out[n-1].EndAt = e.EndAt
			continue
		}
		out = append(out, e)
	}
	return out
}

func hasLength(e Entry) bool {
	return e.EndAt.After(e.StartAt)
}

func earliest(a, b time.Time) time.Time {
	if a.Before(b) {
		return a
	}
	return b
}

func latest(a, b time.Time) time.Time {
	if a.After(b) {
		return a
	}
	return b
}
