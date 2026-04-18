// Package flagpair provides a thin wrapper over the standard library flag
// package that supports paired long/short flags (e.g. "-d, --directory") and
// renders grouped help output in declaration order.
//
// It targets small CLIs that want POSIX-style short/long flags without
// pulling in an external dependency like spf13/pflag or spf13/cobra.
package flagpair

import (
	"flag"
	"fmt"
	"strings"
	"time"
)

// Set wraps a flag.FlagSet and tracks long/short flag pairs so that help
// output can render them grouped on a single line.
type Set struct {
	fs    *flag.FlagSet
	pairs []pair
}

type pair struct {
	long, short string // short may be empty
}

// New creates a Set using flag.ContinueOnError.
func New(name string) *Set {
	s := &Set{fs: flag.NewFlagSet(name, flag.ContinueOnError)}
	s.fs.Usage = s.printUsage
	return s
}

// FlagSet returns the underlying flag.FlagSet for escape-hatch access (e.g.
// SetOutput). Flags registered directly on it will not appear in grouped
// usage output.
func (s *Set) FlagSet() *flag.FlagSet { return s.fs }

// String registers a string flag. Pass short="" to skip the short alias.
func (s *Set) String(long, short, def, usage string) *string {
	p := s.fs.String(long, def, usage)
	if short != "" {
		s.fs.StringVar(p, short, def, usage)
	}
	s.pairs = append(s.pairs, pair{long, short})
	return p
}

// Int registers an int flag.
func (s *Set) Int(long, short string, def int, usage string) *int {
	p := s.fs.Int(long, def, usage)
	if short != "" {
		s.fs.IntVar(p, short, def, usage)
	}
	s.pairs = append(s.pairs, pair{long, short})
	return p
}

// Bool registers a bool flag.
func (s *Set) Bool(long, short string, def bool, usage string) *bool {
	p := s.fs.Bool(long, def, usage)
	if short != "" {
		s.fs.BoolVar(p, short, def, usage)
	}
	s.pairs = append(s.pairs, pair{long, short})
	return p
}

// Duration registers a time.Duration flag.
func (s *Set) Duration(long, short string, def time.Duration, usage string) *time.Duration {
	p := s.fs.Duration(long, def, usage)
	if short != "" {
		s.fs.DurationVar(p, short, def, usage)
	}
	s.pairs = append(s.pairs, pair{long, short})
	return p
}

// Var registers a flag.Value. Both names share the same value.
func (s *Set) Var(value flag.Value, long, short, usage string) {
	s.fs.Var(value, long, usage)
	if short != "" {
		s.fs.Var(value, short, usage)
	}
	s.pairs = append(s.pairs, pair{long, short})
}

// Parse parses args. Returns flag.ErrHelp on -h or --help.
func (s *Set) Parse(args []string) error {
	return s.fs.Parse(args)
}

func (s *Set) printUsage() {
	w := s.fs.Output()
	fmt.Fprintf(w, "Usage of %s:\n", s.fs.Name())

	type row struct {
		head, usage, def string
	}
	const helpHead = "-h, --help"
	rows := make([]row, 0, len(s.pairs)+1)
	headWidth := len(helpHead)

	for _, p := range s.pairs {
		f := s.fs.Lookup(p.long)
		typeName, usage := flag.UnquoteUsage(f)

		var names []string
		if p.short != "" {
			names = append(names, "-"+p.short)
		}
		names = append(names, "--"+p.long)
		head := strings.Join(names, ", ")
		if typeName != "" {
			head += " " + typeName
		}

		var def string
		if f.DefValue != "" && f.DefValue != "false" && f.DefValue != "0" {
			def = fmt.Sprintf(" (default %q)", f.DefValue)
		}

		rows = append(rows, row{head, usage, def})
		if len(head) > headWidth {
			headWidth = len(head)
		}
	}
	rows = append(rows, row{helpHead, "Show this help", ""})

	for _, r := range rows {
		fmt.Fprintf(w, "  %-*s  %s%s\n", headWidth, r.head, r.usage, r.def)
	}
}
