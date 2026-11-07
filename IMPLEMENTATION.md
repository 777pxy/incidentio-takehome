# Implementation notes

Implementation details behind the decisions in [DESIGN.md](DESIGN.md).

## Converting times to UTC
Every time is converted to UTC once, as soon as it's parsed. As well as keeping the output consistent, this means times compare the same way everywhere. Go's `==` and testify's `assert.Equal` both treat the same moment in two different zones as different. Merging uses `.Equal`, which ignores zones, so it works either way, but the tests rely on the conversion.

The original offset is lost, but nothing needs it as shifts are a fixed number of 24 hour periods. Shifts defined in local time would need proper time zone handling (see Working Hours in DESIGN.md).

## The 1 year window check
The check uses the `--from` given by the user, before it's clamped to `handover_start_at`. A year is a calendar year, so it handles leap years.

One quirk: a `--from` on 29th February allows a 366-day window. Go's `AddDate` moves 29th February plus 1 year to 1st March, because the next year has no 29th February.

## The 100 year --from check
Go's durations can only hold about 292 years, so past that the gap between `handover_start_at` and `--from` stops growing and the wrong shift and user would be picked. The 100 year cap stays well clear of that.

The check uses the `--from` given by the user. A year is a calendar year.

## How encoding/json reads and writes JSON
- A time of `null` or `0001-01-01T00:00:00Z` is reported as "is missing". Go uses `0001-01-01T00:00:00Z` as its empty time, so the two can't be told apart from a missing value.
- Keys are matched without regard to case, so `"USERS"` is read as `users`. This is how Go's `encoding/json` works.
- If a key appears twice in the same object, e.g. `{"users": ["a"], "users": ["b"]}`, the last value is used and no error is given. This is also how Go's `encoding/json` works.
- A file starting with a UTF-8 byte order mark (BOM), which some Windows editors add, is rejected as malformed JSON: `invalid character '\ufeff' looking for beginning of value`.
- HTML characters in names are escaped in the output, so `<a&b>` comes out as `"\u003ca\u0026b\u003e"`. It's still valid JSON and decodes back to `<a&b>`.

## Cobra
I used [Cobra](https://github.com/spf13/cobra) for the command line rather than Go's built-in `flag` package.
- It's the most widely used CLI library for Go, so it's well trusted in production.
- A new team-mate will likely recognise it.

## testify
I used [testify](https://github.com/stretchr/testify) for tests rather than only Go's built-in `testing` package.
- I'm a lot more familiar with it, so I can read, check and change the tests with confidence.
- `assert.Equal` shows a clear diff when it fails, which matters when comparing whole lists of entries. With only `testing`, every check needs its own `if` and `t.Errorf`, and `reflect.DeepEqual` doesn't say what differed. I also don't find that style as clean to read or write.
- `require` stops a test at the first failure where carrying on makes no sense, e.g. when the binary fails to build.
- It's widely used, so the tests read the same as most other Go projects.

## JSON library
I used Go's standard `encoding/json` rather than the newer `encoding/json/v2`.
- Everyone knows it, so a new team-mate or reviewer will almost certainly have used it, and there are far more examples and answers for it.
- It's proven. It has been used in production for over a decade, so its quirks are well known and documented.
- The brief doesn't need the extra strictness. Its priority is a correct schedule from valid input, and what `json/v2` adds is mostly catching mistakes in hand-edited files.
- `json/v2` matches keys case-sensitively and rejects duplicate keys by default, but having that built in doesn't outweigh the familiarity and support `encoding/json` has.
- `encoding/json` can still catch misspelled keys with `DisallowUnknownFields`, which is the gap that matters most. Case-insensitive and duplicate keys are documented above as known behaviour.
