# Building:
- Scheduler that rotates people and handles overrides.
- Shifts share the same period = `handover_interval_days`

## Inputs:
- `--schedule` JSON file containing a definition of a schedule (see above example)
- `--overrides` JSON file containing an array of overrides (see above example)
- `--from` the time from which to start listing entries
- `--until` the time until which to list entries

## Output:
- JSON array of the final schedule

Safe assumptions:
- Overrides dont overlap with other overrides
- Overrides will always be after a schedule's `handover_start_at`

## Config Shape:
```js
// This is a schedule.
{
  "users": [
    "alice",
    "bob",
    "charlie"
  ],

  // 5pm, Friday 6th November 2026
  "handover_start_at": "2026-11-06T17:00:00Z",
  "handover_interval_days": 7
}

```

## Override Shape:
```js
// This is an override.
{
  // Charlie will cover this shift
  "user": "charlie",
  // 5pm, Monday 9th November 2026
  "start_at": "2026-11-09T17:00:00Z",
  // 10pm, Monday 9th November 2026
  "end_at": "2026-11-09T22:00:00Z"
}
```

# Algorithm:

## Assumptions:
- Schedule array comes sorted as we get a config file and I can guarantee a sorted schedule pre-override.

## Schedule Generation
Using handover_start_at as an anchor I would calculate how many shifts have occured between handover and `--from`.

shift_no = floor((from - handover) / period)
first_user_on_shift = users[shift_no % len(users)]

first_shift_start = shift_no x period + handover
first_shift_end = first_shift_start + period
actual_first_shift_start = `--from`

From there it would be a matter of cycling through users (stop condition is when start >= `--until`) and having the end of each shift be the start of the next adding shift_period each time to start to calculate the end of each shift. Then truncate the final shift to `--until`


## Main Idea
Sort the overrides based on time (Golang uses pdqsort so worst case is O(VlogV)).

For each override:
1. Output every shift that ends at or before the override starts.
2. For each shift that overlaps the override, keep only the parts outside of it:
- left part: [shift.start, override.start]
- right part: [override.end, shift.end]
Parts with <= 0 lengths are dropped. This covers all cases e.g. splits, trimmed start/ends, and full replacement as it assumes the shifts overlap (which step 1 guarantees).
3. Output the left part.
4. Output the override once (at the first shift it overlaps)
5. Keep the right part as the current shift as the next override may cut into it too.

After all overrides, output the remaining shifts and then truncate the output based on `--from` and `--until` reusing the length <= 0 logic from earlier. Finally, merge any touching entries with the same user.

### Time Complexity
S = number of shifts
V = number of overrides

Every step moves at least one of pointers forwards, so the walk is O(S + V). Sorting the overrides is O(VlogV) so all together it's O(S + VlogV) and given that the max number of overrides is < 100 this essentially costs nothing. In practice the shifts would dominate.

## Edge Cases
### `--until` <= handover_start
Return an empty array.

### `--from` < handover_start
Apply max(`--from`, handover_start).

### `--until` <= `--from`
Return an empty array.

### Overrides before or after `--from` or `--until`
These get handled implicitly by the truncation.

### Mergers
If any two touching entries have the same user (name technically) they will be merged into one shift.

Although, this does loses information on if the shift comes from an override or an actual shift.

### Invalid Input
Note: It was stated that overrides will not start before the schedule so there's no point worrying about them.

Invalid inputs in this context would only be comprised of 2 types:
1. Malformed - input cannot be parsed
2. Parseable - input is parseable but it does not contain the right/enough information.

Both of these should print a detailed error to stderr and exit in a non-zero state.

# Potential Future Features:

## Product Features
### Escalation Layers
Stack schedules into tiers, e.g. primary, secondary, then a manager.
Each tier can reuse the existing scheduler with its own user list, but
new logic and infrastructure would be needed to detect when a page
isn't acknowledged in time and escalate to the next tier.

### UI
In general, some kind of GUI would increase usability.
This could also be used to enforce specific rules e.g. no overrides before the handover, no shifts longer than x days, minimum period length etc.

A lot of these other features would generally fall under UI.

### Working Hours
If a company spreads its responders across multiple time zones, it may want each team to only cover their daytime hours as to ensure whoever is on call is awake and alert.

This would mean adding each user's timezone and working hours to the config (or retrieved from a database) so the scheduler (or UI) can block anyone from being scheduled outside their working hours.

Shifts would now be able to be defined in any local time but this messes with the scheduling maths due to daylight savings so it would have to be updated.

### Slivers
If an override results in a small sliver of a shift being left notify the user creating the override and suggest extending it providing an easy way to do so.

### On-call hours, pay tracker, and work balance
Hours per user would be tracked accordingly e.g. weekday, weekend and bank holiday.
This could then be used to calculate the on-call pay for each user. It would also provide insight into who is working the most and who could pick up shifts to make sure the workload is fair.

### Reminders / Calender plugins
Notify people via slack, teams etc of their upcoming shift or create a plugin so users can export their schedules to a calender.

## The Tool

### `--at`
Who's on call at the specific given time and who to escalate to.
Could also return information on that user's shift e.g. start, end times.

It would be relatively easy to implement as a combination of the logic behind `--from` and `--until` or more efficiently with its own indexing logic.

### `--user`
Only one person's shifts i.e. "my shifts".

### `.ics` export
This is essentially the Reminders / Calenders plugin mentioned in Product Features.

### Lint Mode
This would comb over the schedule and flag any slivers, overlapping overrides rather than just blindly trusting user inputs.

# Limits:

Generally, schedules and overrides would never reach a size where the algorithm would be the limiting factor especialy since calculation costs are based on the size of the requested window.

I think the real limitations right now are:
1. No shared source of truth amongst teammates
The only way to share a schedule currently is via cloud storage. There's no history, race condition blockers, invalid input checks etc.

The fix is a backend that owns the data acting as a single source of truth, and all operations made through an API. This would ensure validation on every operation as well as an audit log

2. Recomputation
Schedules are re-rendered on every request.
Caching would avoid repeated work but must ensure that schedules are invalidated whenver an override or shifts change.

However, at this scale / trivial size I would have to weight the benefits of the saved computation costs from caching with the calculation costs of caching.

3. Irrelevant Overrides
Find the relevant overrides only via `override.start` `override.end`. This would avoid wasted computation.

4. Identification
Users are referenced only by name which isn't unique.
Using user ID's provides enough unique information to find contact information, and would support the development of a UI.

5. Paging Lookup
If an issue arises it wont just be one person being paged. Multiple teams would be getting notified which could fire hundreds of alerts.

If the lookup for who's on call fails nobody gets paged apart from possibly the team responsible for the scheduling.

Using a lookup system that instead just checks that specific moment in time and for any overrides would reduce the chances that something goes wrong as it avoids a full schedule render.