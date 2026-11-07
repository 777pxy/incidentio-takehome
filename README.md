# render-schedule

A scheduler that rotates people through on-call shifts and handles overrides. Shifts all share the same period, `handover_interval_days`. It prints the final schedule between `--from` and `--until` as a JSON array.

## Contents
- This README: [Requirements](#requirements), [Build](#build), [Run](#run), [Test](#test), [Project layout](#project-layout)
- [DESIGN.md](DESIGN.md): how it works and why
  - [Algorithm](DESIGN.md#algorithm)
  - [Edge Cases](DESIGN.md#edge-cases)
  - [Validation](DESIGN.md#validation)
  - [Potential Future Features](DESIGN.md#potential-future-features)
  - [Limits](DESIGN.md#limits)
- [IMPLEMENTATION.md](IMPLEMENTATION.md): the implementation details behind those decisions
  - [Converting times to UTC](IMPLEMENTATION.md#converting-times-to-utc)
  - [The 1 year window check](IMPLEMENTATION.md#the-1-year-window-check)
  - [The 100 year --from check](IMPLEMENTATION.md#the-100-year---from-check)
  - [How encoding/json reads and writes JSON](IMPLEMENTATION.md#how-encodingjson-reads-and-writes-json)
  - [Cobra](IMPLEMENTATION.md#cobra)
  - [testify](IMPLEMENTATION.md#testify)
  - [JSON library](IMPLEMENTATION.md#json-library)

## Requirements
- Go 1.23 or later ([install Go](https://go.dev/doc/install)).

The only dependencies are [Cobra](https://github.com/spf13/cobra) for the command line and [testify](https://github.com/stretchr/testify) for tests. Go downloads them automatically on the first build or test, or you can fetch them up front with:

```console
go mod download
```

## Build
From the repo root:

```console
go build -o render-schedule .
```

### Install globally (optional)
To run `render-schedule` from anywhere, build it into Go's bin folder instead:

```console
go build -o "$(go env GOPATH)/bin/render-schedule" .
```

This needs Go's bin folder (usually `~/go/bin`) on your `PATH`. If `render-schedule` isn't found, add it with:

```console
export PATH="$PATH:$(go env GOPATH)/bin"
```

## Run
```console
./render-schedule \
    --schedule=testdata/brief/schedule.json \
    --overrides=testdata/brief/overrides.json \
    --from='2026-11-06T17:00:00Z' \
    --until='2026-11-20T17:00:00Z'
```

Output:

```json
[
  {
    "user": "alice",
    "start_at": "2026-11-06T17:00:00Z",
    "end_at": "2026-11-09T17:00:00Z"
  },
  {
    "user": "charlie",
    "start_at": "2026-11-09T17:00:00Z",
    "end_at": "2026-11-09T22:00:00Z"
  },
  {
    "user": "alice",
    "start_at": "2026-11-09T22:00:00Z",
    "end_at": "2026-11-13T17:00:00Z"
  },
  {
    "user": "bob",
    "start_at": "2026-11-13T17:00:00Z",
    "end_at": "2026-11-20T17:00:00Z"
  }
]
```

Entries that cross `--from` or `--until` are cut to the window.

### Inputs
All four flags are required.
- `--schedule` JSON file containing a definition of a schedule (see below)
- `--overrides` JSON file containing an array of overrides (see below)
- `--from` the time from which to start listing entries
- `--until` the time until which to list entries

Times are in RFC 3339 format, e.g. `2026-11-06T17:00:00Z`. An offset such as `+01:00` in place of the `Z` is also accepted. Output times are always in UTC.

Schedule shape:
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

Override shape (the overrides file is an array of these):
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

Safe assumptions, from the brief:
- Overrides don't overlap with other overrides
- Overrides will always be after a schedule's `handover_start_at`

These are still checked, and broken ones are rejected (see below).

### Never trust the input
I've taken a zero-trust approach to input: never trust the user, because they're guaranteed to get something wrong in some unpredictable way. So every input is validated, even what the brief says will never happen, and anything that looks like a mistake is rejected with a clear error. An error is easy to spot and fix, but a plausible-looking wrong schedule means someone doesn't get paged.

### Errors
Invalid input prints a detailed error to stderr and exits with code 1, e.g.:

```console
$ ./render-schedule --schedule=testdata/invalid/schedule.json ...
render-schedule: testdata/invalid/schedule.json: invalid schedule:
  - users: must contain at least one user
  - handover_interval_days: must be at least 1
```

Overrides that break the brief's assumptions are named by their place in the file, their user and their start time:

```console
$ ./render-schedule --overrides=overrides.json ...
render-schedule: overrides.json: invalid overrides:
  - overrides[2] (alice, starting 2026-11-05T09:00:00Z): must not start before handover_start_at (2026-11-06T17:00:00Z)
  - overrides[1] (bob, starting 2026-11-09T20:00:00Z): overlaps overrides[0] (charlie, starting 2026-11-09T17:00:00Z)
```

## Test
```console
go test ./...
```

The end-to-end tests in `main_test.go` build the binary and run it twice:
- on the brief's example, comparing the output exactly to `testdata/brief/expected.json`
- on the invalid schedule in `testdata/invalid/`, checking it exits with code 1 and prints the exact error to stderr and nothing to stdout

## Project layout
- `main.go` runs the command and exits with its exit code.
- `cmd/root.go` defines the flags and loads and validates the input.
- `internal/oncall/` holds the scheduler:
  - `types.go` defines `Schedule`, `Override` and `Entry`.
  - `parse.go` decodes and validates the JSON files.
  - `generate.go` generates the shifts.
  - `render.go` applies the overrides, truncates to the window and merges entries.
- Tests sit next to the code they test, in `_test.go` files. `main_test.go` holds the end-to-end tests.
- `testdata/` holds the input files used by the tests.
