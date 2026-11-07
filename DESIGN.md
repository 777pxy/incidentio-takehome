# Design

How `render-schedule` works and why. For how to build and run it, see [README.md](README.md). For the implementation details behind these decisions, see [IMPLEMENTATION.md](IMPLEMENTATION.md).

# Algorithm

## Assumptions
- Generated shifts are already in time order, as Schedule Generation produces them one after another, so the shifts are sorted before any overrides are applied.

## Schedule Generation
Using handover_start_at as an anchor, it calculates how many shifts have occurred between the handover and `--from`.

`--from` is clamped to handover_start_at first (see Edge Cases). This matters because Go's integer division rounds towards zero, so it only acts as a floor when `from - handover` isn't negative.

```
shift_no = floor((from - handover) / period)
first_user_on_shift = users[shift_no % len(users)]

first_shift_start = shift_no x period + handover
first_shift_end = first_shift_start + period
actual_first_shift_start = --from
```

From there, it cycles through the users, with each shift starting where the previous one ended and ending one period later. It stops once a shift would start at or after `--until`, and the final shift is cut to `--until`.

## Main Idea
The overrides arrive sorted by time, as they're sorted once when they're checked (see Validation). Go uses pdqsort so worst case is O(VlogV).

For each override:
1. Output every shift that ends at or before the override starts.
2. For each shift that overlaps the override, keep only the parts outside of it:
   - left part: [shift.start, override.start]
   - right part: [override.end, shift.end]

   Parts with <= 0 lengths are dropped. This covers all cases, e.g. splits, trimmed starts and ends, and full replacement. It relies on the shift overlapping the override: step 1 guarantees each shift ends after the override starts, and the step 2 condition guarantees it starts before the override ends.
3. Output the left part.
4. Output the override once (at the first shift it overlaps)
5. Keep the right part as the current shift as the next override may cut into it too.

After all overrides, output the remaining shifts and then truncate the output based on `--from` and `--until` reusing the length <= 0 logic from earlier. Finally, merge any touching entries with the same user.

## Time Complexity
S = number of shifts
V = number of overrides

Every step moves at least one of the pointers forwards, so the walk is O(S + V). Sorting the overrides is O(VlogV), and checking them afterwards is O(V), so all together it's O(S + VlogV) and given that the max number of overrides is < 100 this essentially costs nothing. In practice the shifts would dominate.

# Edge Cases
## --until <= handover_start
Return an empty array.

## --from < handover_start
Apply max(`--from`, handover_start).

## --until <= --from
Return an empty array.

## Overrides before or after --from or --until
An override that crosses `--from` or `--until` is cut to the window by the truncation.

An override entirely outside the window overlaps no shift, so step 4 never outputs it and it's dropped.

## Merging
If two touching entries have the same user (by name), they're merged into one entry.

This does lose whether that time came from an override or a regular shift.

## Same user more than once in users
Duplicate names are allowed, e.g. for weighted rotations where someone takes more turns than others (`["alice", "bob", "alice"]`).

A repeated name is treated as the same person, because the name is the user's identifier. This means two different people both called Alice can't be told apart. Doing so needs user IDs, which is covered in Limits under Identification.

## Time zones
The brief doesn't say what time zone inputs will be in. Every example ends in `Z` (UTC), but RFC 3339 also allows an offset in place of the `Z`, e.g. `2026-11-06T18:00:00+01:00`.

I decided to accept any offset rather than only `Z`, as rejecting a valid RFC 3339 time would be assuming more than the brief says, and would be surprising to someone who copied a time from a tool that outputs local offsets. The offset has to be in `±HH:MM` form, e.g. `+01:00` or `-05:00`, so `+0100` is rejected.

Every time is converted to UTC as soon as it's parsed. This only changes how a time is displayed, not the moment it refers to, so the shifts come out the same. The output is consistent, with every time ending in `Z`, instead of mixing the offsets from the inputs. See IMPLEMENTATION.md for more on the conversion.

# Validation
The brief's safe assumptions (overrides don't overlap, and don't start before `handover_start_at`) are still checked, as the user can't be trusted to follow them (see the README). If they were broken, the algorithm would still produce output, but it wouldn't be a valid schedule:
- Overlapping overrides would produce entries that overlap in the output.
- An override entirely inside another override would be dropped.
- An override before `handover_start_at` would produce an entry before the rotation starts.

So instead, the overrides are sorted by start time and then checked in that order. Each one is compared with the override that ends latest so far, which also catches one sitting inside an earlier, longer override. Any that break the assumptions are rejected with an error, e.g. `overrides[1] (bob, starting 2026-11-09T20:00:00Z): overlaps overrides[0] (charlie, starting 2026-11-09T17:00:00Z)`. The sorted overrides are then passed on to the Main Idea, so they're only sorted once.

Invalid inputs in this context fall into 2 types:
1. Malformed - input cannot be parsed
2. Parseable - input is parseable but it does not contain the right/enough information.

Both of these print a detailed error to stderr and exit with a non-zero code.

## Window longer than 1 year
If `--until` is more than 1 year after `--from`, print an error and exit with a non-zero code.

This decision was based on the brief, which says the code will only be run for time windows of up to a few weeks. A window over a year is far outside that, so it's most likely a mistake, e.g. typing the wrong year, and rendering it would produce a long list nobody asked for. A year still leaves plenty of room above a few weeks.

## handover_interval_days longer than 1 year
If `handover_interval_days` is more than 365, print an error and exit with a non-zero code.

I don't expect anyone to be on call for more than a month, let alone a year, so a 1 year cap leaves more than enough room. Anything longer must be a user input error.

## --from more than 100 years after handover_start_at
If `--from` is more than 100 years after `handover_start_at`, print an error and exit with a non-zero code.

Past a few hundred years the wrong shift and user would be picked (see IMPLEMENTATION.md). No real schedule runs from the same anchor for 100 years, so the cap stays well clear of that and catches typos like `3026` instead of `2026`. It's 100 rather than 1 year because a rotation can run for years from the same anchor.

## Other rules
- All four flags are required, file paths can't be empty, and unknown flags or extra arguments are rejected.
- `users` must have at least one user, and no name can be blank.
- `handover_start_at` must be present, and `handover_interval_days` must be at least 1.
- Each override needs a non-blank `user`, a `start_at` and an `end_at`, with `end_at` after `start_at`.
- No override can start before `handover_start_at` or overlap another override. Touching overrides, where one ends exactly when the next starts, are allowed.
- Unknown keys are rejected, e.g. `unknown key "handover_intervl_days"`. Without this, a misspelled key would be ignored and only show up as a missing value, e.g. `handover_interval_days: must be at least 1`, which doesn't say the name was wrong. The error doesn't say which override the key was in.
- JSON syntax and type errors stop at the first one and give its line and column. Once a file decodes, every validation problem in it is reported at once. The one exception is that overlaps and overrides before the handover are only checked once every override is valid on its own, as they compare overrides with each other and with the schedule. Reporting every decoding error as well would mean decoding each field separately, which is too exhaustive for the benefit.
- Bad timestamps in a file are caught by Go's time parser while decoding. The error shows the bad value and the expected format, e.g. `timestamp "6th Nov 2026" must be in RFC 3339 format ...`.

For how `encoding/json` treats `null` times, key case, duplicate keys, byte order marks and HTML characters, see IMPLEMENTATION.md.

# Potential Future Features

## Product Features
### Escalation Layers
Stack schedules into tiers, e.g. primary, secondary, then a manager.
Each tier can reuse the existing scheduler with its own user list, but
new logic and infrastructure would be needed to detect when a page
isn't acknowledged in time and escalate to the next tier.

This is slightly out of scope for now, as the tool is just a CLI with no connectivity: it can't send pages or know whether one was acknowledged.

Real schedules also stack rotations within a tier, e.g. a shadow rotation for someone learning the ropes, so there would need to be a rule for which layer wins when they overlap.

### UI
In general, some kind of GUI would increase usability.
This could also be used to enforce specific rules e.g. no overrides before the handover, no shifts longer than x days, minimum period length etc.

A lot of these other features would generally fall under UI.

### Working Hours
If a company spreads its responders across multiple time zones, it may want each team to only cover their daytime hours so as to ensure whoever is on call is awake and alert.

This would mean adding each user's timezone and working hours to the config (or retrieved from a database) so the scheduler (or UI) can block anyone from being scheduled outside their working hours.

Shifts would now be able to be defined in any local time but this messes with the scheduling maths due to daylight savings so it would have to be updated.

### Local time and daylight saving
Shifts are currently fixed 24 hour blocks in UTC, but people think in local time, e.g. "handover at 5pm London". That's 17:00 UTC in winter and 16:00 UTC in summer, so a handover set in UTC drifts by an hour twice a year.

Storing a time zone with each schedule and counting local calendar days would keep handovers at the same local time all year.

### Time off
Holidays and time off can clash with a shift. Users could mark when they're away so they're skipped, or so the clash is flagged before the shift starts.

### Slivers
If an override results in a small sliver of a shift being left, notify the user creating the override and suggest extending it, providing an easy way to do so, as being on shift for 5 minutes is a bit ridiculous.

### On-call hours, pay tracker, and work balance
Hours per user would be tracked accordingly e.g. weekday, weekend and bank holiday.
This could then be used to calculate the on-call pay for each user. It would also provide insight into who is working the most and who could pick up shifts to make sure the workload is fair.

### Reminders / Calendar plugins
Notify people via Slack, Teams etc of their upcoming shift or create a plugin so users can export their schedules to a calendar.

## The Tool

### --at
Who's on call at the specific given time and who to escalate to.
Could also return information on that user's shift e.g. start, end times.

It would be relatively easy to implement as a combination of the logic behind `--from` and `--until` or more efficiently with its own indexing logic.

### --user
Only one person's shifts i.e. "my shifts".

### .ics export
This is essentially the Reminders / Calendar plugins mentioned in Product Features.

### Lint Mode
This would comb over the schedule and flag any slivers rather than just blindly trusting user inputs.

It could also flag gaps, where nobody is on call, and conflicts, where someone is on two shifts at once.

Overlapping overrides are already rejected, but a real product may want a softer rule, e.g. letting one take priority, rather than rejecting the whole file.

# Limits

Generally, schedules and overrides would never reach a size where the algorithm would be the limiting factor, especially since calculation costs are based on the size of the requested window.

I think the real limitations right now are:
1. No shared source of truth amongst teammates

   The only way to share a schedule currently is by passing the JSON files around, e.g. via cloud storage. There's no history, nothing to stop two people overwriting each other's changes, and no checks when several people edit the shared files.

   The fix is a backend that owns the data acting as a single source of truth, and all operations made through an API. This would ensure validation on every operation as well as an audit log.

2. Recomputation

   Schedules are re-rendered on every request.
   Caching would avoid repeated work but must ensure that schedules are invalidated whenever an override or shifts change.

   However, at this scale the computation saved would be tiny, so I would have to weigh it against the extra work of storing the cache and keeping it up to date.

3. Irrelevant Overrides

   Every override in the file is read and sorted, even though only the ones that overlap the window matter. With a database, a query on `start_at` and `end_at` could fetch just those. This would avoid wasted computation.

4. Identification

   Users are referenced only by name which isn't unique.
   Using user IDs provides enough unique information to find contact information, and would support the development of a UI.

   Shifts and overrides have no IDs either. Swaps, notifications and audit logs all need to point at a specific shift, and overrides should record who made them and why.

5. Paging Lookup

   Paging only needs to know who's on call at one moment, but the only way to find that out is a full schedule render, which is heavier and has more that can go wrong.

   That matters because when an issue arises it won't just be one person being paged. Multiple teams would be getting notified which could fire hundreds of alerts, and if the lookup for who's on call fails nobody gets paged apart from possibly the team responsible for the scheduling.

   Using a lookup system that instead just checks that specific moment in time and any overrides covering it would reduce the chances that something goes wrong.

6. Changing the rotation rewrites history

   Who's on call is worked out from the anchor, as `users[shift_no % len(users)]`. Adding or removing a single user reshuffles every shift, including ones that have already happened.

   Changes should apply from a date onwards instead, and past shifts should be kept as fixed records, since on-call pay, audits and "who got paged at 3am?" all depend on them.
