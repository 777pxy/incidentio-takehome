# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project status

This is the incident.io early-careers take-home, written in Go with Cobra. It is currently a hello-world skeleton: the command prints a greeting and has no flags, JSON parsing or scheduling logic yet.

- `README.md` is the brief and the source of truth for behaviour.
- `starting-notes.md` holds the candidate's planned algorithm, edge cases and draft talking points for the video.

Only implement what the candidate explicitly asks for in the current request.

## Commands

```console
go run .                          # run without building
go build -o render-schedule .     # build the ./render-schedule binary (gitignored)
go vet ./...
go test ./...                     # no tests exist yet
go test ./... -run TestName       # run a single test
```

`main.go` calls `cmd.Execute()`, and the root command is defined in `cmd/root.go`.

## The deliverable

An executable named `./render-schedule`, run from the repo root:

```console
./render-schedule \
    --schedule=schedule.json \
    --overrides=overrides.json \
    --from='2026-11-06T17:00:00Z' \
    --until='2026-11-20T17:00:00Z'
```

It prints a JSON array of `{user, start_at, end_at}` entries: the rotation with overrides applied, truncated to the `--from`/`--until` window. The README's worked example is the natural first test case. The JSON fixtures it uses aren't in the repo yet, but their contents are shown in the README.

The README guarantees that overrides start after `handover_start_at` and don't overlap each other. It does not guarantee that the overrides file is sorted. The brief prioritises correctness over performance (windows of a few weeks, under 100 overrides).

## Planned algorithm

From `starting-notes.md`: generate the shifts in time order, then sweep shifts and overrides together with two pointers. Each override splits an overlapping shift into at most two remainders, and any remainder with positive length is kept:

- `[shift_start, min(shift_end, override_start)]`
- `[max(shift_start, override_end), shift_end]`

The override itself is inserted before the second remainder. An override can span several shifts, so it must keep applying until a shift ends after it.

The notes list "override array comes sorted" as an assumption, but the README doesn't guarantee order, so sort the overrides.

Edge-case decisions in the notes:

- **Merging:** touching entries for the same user are merged into one. The same user can also appear more than once in `users`.
- **Slivers:** small leftover pieces of a shift are kept as they are.
- **Overrides past the end:** these are truncated to the end of the window.
- **Invalid input:** malformed input (can't be parsed) and parseable input with wrong or missing information both produce a detailed error message.
- **`--from` before `handover_start_at`:** this section of the notes is unfinished, so ask the candidate how to handle it. It flags a concern about negative values in Go's integer division when working out which shift a time falls in.

## Submission

- A ZIP containing the code and a `README.md` with run and dependency-install instructions. The current `README.md` is the brief, so it will need replacing or moving.
- A 5-minute video link goes in the submission's notes field, not the repo. It covers the approach and why, how the code implements it, and what features could come next, pitched at a new team-mate.
- AI tools are allowed, but the candidate must be able to explain and justify every decision. Keep the code simple enough to walk through, and surface design decisions to the candidate rather than settling them silently.

## Git

- Commit messages should be concise and not overly verbose.
