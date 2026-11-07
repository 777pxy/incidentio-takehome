// Package cmd defines the render-schedule command line interface.
package cmd

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/777pxy/incidentio-takehome/internal/oncall"
	"github.com/spf13/cobra"
)

// options holds the raw flag values.
type options struct {
	schedulePath  string
	overridesPath string
	from          string
	until         string
}

// input is everything the scheduler needs, parsed and validated.
type input struct {
	schedule  oncall.Schedule
	overrides []oncall.Override
	from      time.Time
	until     time.Time
}

// Run executes the CLI with the given arguments and returns the process exit
// code. Errors are written to stderr.
func Run(args []string, stdout, stderr io.Writer) int {
	root := newRootCmd()
	root.SetArgs(args)
	root.SetOut(stdout)
	root.SetErr(stderr)
	if err := root.Execute(); err != nil {
		fmt.Fprintf(stderr, "render-schedule: %v\n", err)
		return 1
	}
	return 0
}

func newRootCmd() *cobra.Command {
	var opts options
	root := &cobra.Command{
		Use:   "render-schedule --schedule=FILE --overrides=FILE --from=TIME --until=TIME",
		Short: "Render an on-call schedule with overrides applied",
		Args:  cobra.NoArgs,
		// Run prints errors itself, so stop Cobra printing them (and the
		// full usage text) a second time.
		SilenceErrors: true,
		SilenceUsage:  true,
		RunE: func(cmd *cobra.Command, args []string) error {
			in, err := loadInput(opts)
			if err != nil {
				return err
			}
			entries := oncall.Render(in.schedule, in.overrides, in.from, in.until)

			enc := json.NewEncoder(cmd.OutOrStdout())
			enc.SetIndent("", "  ")
			return enc.Encode(entries)
		},
	}

	flags := root.Flags()
	flags.StringVar(&opts.schedulePath, "schedule", "", "path to the schedule JSON file")
	flags.StringVar(&opts.overridesPath, "overrides", "", "path to the overrides JSON file")
	flags.StringVar(&opts.from, "from", "", "start of the window, in RFC 3339 format")
	flags.StringVar(&opts.until, "until", "", "end of the window, in RFC 3339 format")
	for _, name := range []string{"schedule", "overrides", "from", "until"} {
		// Only errors if the flag doesn't exist, which it always does.
		_ = root.MarkFlagRequired(name)
	}
	return root
}

func loadInput(opts options) (input, error) {
	from, err := parseTimeFlag("from", opts.from)
	if err != nil {
		return input{}, err
	}
	until, err := parseTimeFlag("until", opts.until)
	if err != nil {
		return input{}, err
	}
	// The brief only expects windows of up to a few weeks, so a window over a
	// year is almost certainly a mistake, e.g. the wrong year.
	if until.After(from.AddDate(1, 0, 0)) {
		return input{}, fmt.Errorf("--until (%s) must be at most 1 year after --from (%s)",
			until.Format(time.RFC3339), from.Format(time.RFC3339))
	}
	schedule, err := loadFile("schedule", opts.schedulePath, oncall.ParseSchedule)
	if err != nil {
		return input{}, err
	}
	overrides, err := loadFile("overrides", opts.overridesPath, oncall.ParseOverrides)
	if err != nil {
		return input{}, err
	}
	return input{schedule: schedule, overrides: overrides, from: from, until: until}, nil
}

func parseTimeFlag(name, value string) (time.Time, error) {
	t, err := time.Parse(time.RFC3339, value)
	if err != nil {
		return time.Time{}, fmt.Errorf("--%s %q must be in %s", name, value, oncall.TimeFormatHint)
	}
	return t.UTC(), nil
}

// loadFile reads the file given to --flag and parses it with parse.
func loadFile[T any](flag, path string, parse func([]byte) (T, error)) (T, error) {
	var zero T
	if path == "" {
		return zero, fmt.Errorf("--%s must not be empty", flag)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return zero, fmt.Errorf("reading --%s file: %w", flag, err)
	}
	parsed, err := parse(data)
	if err != nil {
		return zero, fmt.Errorf("%s: %w", path, err)
	}
	return parsed, nil
}
