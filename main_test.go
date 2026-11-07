package main

import (
	"bytes"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// buildBinary builds the real render-schedule binary into a temporary folder
// and returns its path.
func buildBinary(t *testing.T) string {
	t.Helper()
	bin := filepath.Join(t.TempDir(), "render-schedule")
	out, err := exec.Command("go", "build", "-o", bin, ".").CombinedOutput()
	require.NoError(t, err, "go build failed:\n%s", out)
	return bin
}

// TestBriefExample runs the binary with the brief's example input and
// compares its output byte for byte with the brief's expected output.
func TestBriefExample(t *testing.T) {
	run := exec.Command(buildBinary(t),
		"--schedule=testdata/brief/schedule.json",
		"--overrides=testdata/brief/overrides.json",
		"--from=2026-11-06T17:00:00Z",
		"--until=2026-11-20T17:00:00Z",
	)
	var stdout, stderr bytes.Buffer
	run.Stdout = &stdout
	run.Stderr = &stderr
	require.NoError(t, run.Run(), "stderr:\n%s", stderr.String())

	want, err := os.ReadFile("testdata/brief/expected.json")
	require.NoError(t, err)
	assert.Equal(t, string(want), stdout.String())
	assert.Empty(t, stderr.String())
}

// TestInvalidInput runs the binary with an invalid schedule and checks the
// real process exits with code 1, prints the exact error to stderr, and
// prints nothing to stdout.
func TestInvalidInput(t *testing.T) {
	run := exec.Command(buildBinary(t),
		"--schedule=testdata/invalid/schedule.json",
		"--overrides=testdata/brief/overrides.json",
		"--from=2026-11-06T17:00:00Z",
		"--until=2026-11-20T17:00:00Z",
	)
	var stdout, stderr bytes.Buffer
	run.Stdout = &stdout
	run.Stderr = &stderr
	err := run.Run()

	var exitErr *exec.ExitError
	require.True(t, errors.As(err, &exitErr), "expected a non-zero exit, got %v", err)
	assert.Equal(t, 1, exitErr.ExitCode())
	assert.Equal(t, "render-schedule: testdata/invalid/schedule.json: invalid schedule:\n"+
		"  - users: must contain at least one user\n"+
		"  - handover_interval_days: must be at least 1\n", stderr.String())
	assert.Empty(t, stdout.String())
}
