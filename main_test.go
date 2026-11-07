package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestBriefExample builds the real binary, runs it with the brief's example
// input, and compares its output byte for byte with the brief's expected
// output.
func TestBriefExample(t *testing.T) {
	bin := filepath.Join(t.TempDir(), "render-schedule")
	build := exec.Command("go", "build", "-o", bin, ".")
	out, err := build.CombinedOutput()
	require.NoError(t, err, "go build failed:\n%s", out)

	run := exec.Command(bin,
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
