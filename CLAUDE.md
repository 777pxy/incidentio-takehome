# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project status

This is the incident.io early-careers take-home, written in Go with Cobra. `./render-schedule` is fully implemented: it parses and validates the input, generates the shifts, applies the overrides, truncates the result to the window, merges touching entries and prints the result as JSON.

- `README.md` is the candidate's spec and the source of truth for behaviour. Code must match it step by step. If the spec is wrong, point to the exact line rather than coding around it.
- `project_guide.md` is the original incident.io brief. Use it for context only.

Only implement what the candidate explicitly asks for in the current request. When they ask a question, answer it and offer the change rather than making it.

## Commands

```console
go build -o render-schedule .     # build the ./render-schedule binary (gitignored)
go vet ./...
go test ./...
go test ./... -run TestName       # run a single test
```

## Usage

```console
./render-schedule \
    --schedule=testdata/brief/schedule.json \
    --overrides=testdata/brief/overrides.json \
    --from='2026-11-06T17:00:00Z' \
    --until='2026-11-20T17:00:00Z'
```

The command prints a JSON array of `{user, start_at, end_at}` entries. Errors are printed to stderr as `render-schedule: …`, and the exit code is 1.

## Architecture

- `main.go` calls `cmd.Run(args, stdout, stderr)` and exits with the code it returns.
- `cmd/root.go`:
  - defines the Cobra command and its four required flags
  - `loadInput` parses the time flags, which are converted to UTC
  - it applies the 1-year window cap, loads the schedule, applies the 100-year `--from` vs `handover_start_at` cap, then loads the overrides
- `internal/oncall/`:
  - `types.go` holds `Schedule`, `Override` and `Entry`.
  - `parse.go` decodes the JSON. Decoding errors give a line and column. Validation problems are collected into a `ValidationError`, and every time is converted to UTC.
  - `generate.go`: `GenerateShifts` clamps `--from` to the handover before the integer division, then steps forward until `--until`.
  - `render.go`: `Render` runs `GenerateShifts`, then `applyOverrides`, then `truncate`, then `mergeTouching`.
    - `applyOverrides` sorts the overrides and walks them alongside the shifts with two pointers, carrying the right part of each shift forward.
    - `truncate` drops entries with length ≤ 0.
- Tests are table-driven and use testify.
  - `main_test.go` builds the binary and runs it end to end.
  - It compares the output with `testdata/brief/expected.json`, and checks the error output using `testdata/invalid/`.

## Submission

- A ZIP containing the code and a `README.md` with run and dependency-install instructions. The README doesn't have these instructions yet.
- A 5-minute video link goes in the submission's notes field, not the repo. It covers the approach and why, how the code implements it, and what features could come next, pitched at a new team-mate.
- AI tools are allowed, but the candidate must be able to explain and justify every decision. Keep the code simple enough to walk through, and surface design decisions to the candidate rather than settling them silently.

## Git

- Use [Conventional Commits](https://www.conventionalcommits.org/): `type: summary`, e.g. `feat:`, `fix:`, `test:`, `refactor:`, `docs:`, `chore:`.
- Commit messages should be concise and not overly verbose.
