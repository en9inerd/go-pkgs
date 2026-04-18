package flagpair

import (
	"bytes"
	"errors"
	"flag"
	"io"
	"strings"
	"testing"
	"time"
)

func TestParseLongAndShortSharePointer(t *testing.T) {
	s := New("test")
	s.FlagSet().SetOutput(io.Discard)
	dir := s.String("directory", "d", "default", "session dir")
	verbose := s.Bool("verbose", "v", false, "verbose logging")
	limit := s.Int("limit", "l", 0, "row limit")

	cases := []struct {
		args      []string
		wantDir   string
		wantVerb  bool
		wantLimit int
	}{
		{[]string{"--directory", "/long"}, "/long", false, 0},
		{[]string{"-d", "/short"}, "/short", false, 0},
		{[]string{"--verbose"}, "default", true, 0},
		{[]string{"-v"}, "default", true, 0},
		{[]string{"--limit", "42"}, "default", false, 42},
		{[]string{"-l", "7"}, "default", false, 7},
	}

	for _, tc := range cases {
		t.Run(strings.Join(tc.args, " "), func(t *testing.T) {
			*dir = "default"
			*verbose = false
			*limit = 0
			if err := s.Parse(tc.args); err != nil {
				t.Fatalf("parse %v: %v", tc.args, err)
			}
			if *dir != tc.wantDir {
				t.Errorf("dir = %q, want %q", *dir, tc.wantDir)
			}
			if *verbose != tc.wantVerb {
				t.Errorf("verbose = %v, want %v", *verbose, tc.wantVerb)
			}
			if *limit != tc.wantLimit {
				t.Errorf("limit = %d, want %d", *limit, tc.wantLimit)
			}
		})
	}
}

func TestLongOnlyFlag(t *testing.T) {
	s := New("test")
	s.FlagSet().SetOutput(io.Discard)
	port := s.String("port", "", "8080", "listen port")

	if err := s.Parse([]string{"--port", "9090"}); err != nil {
		t.Fatalf("parse: %v", err)
	}
	if *port != "9090" {
		t.Errorf("port = %q, want %q", *port, "9090")
	}

	if s.FlagSet().Lookup("") != nil {
		t.Error(`empty short name should not register a flag`)
	}
}

func TestHelpReturnsErrHelp(t *testing.T) {
	s := New("test")
	var buf bytes.Buffer
	s.FlagSet().SetOutput(&buf)
	s.String("directory", "d", "", "session dir")

	err := s.Parse([]string{"-h"})
	if !errors.Is(err, flag.ErrHelp) {
		t.Fatalf("expected flag.ErrHelp, got %v", err)
	}

	out := buf.String()
	wantFragments := []string{
		"Usage of test:",
		"-d, --directory",
		"session dir",
		"-h, --help",
		"Show this help",
	}
	for _, frag := range wantFragments {
		if !strings.Contains(out, frag) {
			t.Errorf("usage output missing %q\n---\n%s", frag, out)
		}
	}
}

func TestUsageDeclarationOrder(t *testing.T) {
	s := New("test")
	var buf bytes.Buffer
	s.FlagSet().SetOutput(&buf)

	s.String("zulu", "z", "", "z flag")
	s.String("alpha", "a", "", "a flag")
	s.String("mike", "m", "", "m flag")

	_ = s.Parse([]string{"-h"})
	out := buf.String()

	posZulu := strings.Index(out, "--zulu")
	posAlpha := strings.Index(out, "--alpha")
	posMike := strings.Index(out, "--mike")

	if !(posZulu < posAlpha && posAlpha < posMike) {
		t.Errorf("flags not in declaration order (zulu, alpha, mike):\n%s", out)
	}
}

func TestDefaultValueRendering(t *testing.T) {
	s := New("test")
	var buf bytes.Buffer
	s.FlagSet().SetOutput(&buf)

	s.String("dir", "d", "/tmp", "directory")
	s.String("empty", "e", "", "empty default")
	s.Bool("flag", "f", false, "a bool")
	s.Int("zero", "z", 0, "zero default")
	s.Int("count", "c", 5, "non-zero default")

	_ = s.Parse([]string{"-h"})
	out := buf.String()

	if !strings.Contains(out, `(default "/tmp")`) {
		t.Errorf("missing /tmp default: %s", out)
	}
	if !strings.Contains(out, `(default "5")`) {
		t.Errorf("missing count default: %s", out)
	}
	if strings.Contains(out, `(default "")`) {
		t.Errorf("empty default should not be rendered: %s", out)
	}
	if strings.Contains(out, `(default "false")`) {
		t.Errorf("false default should not be rendered: %s", out)
	}
	if strings.Contains(out, `(default "0")`) {
		t.Errorf("zero default should not be rendered: %s", out)
	}
}

func TestDurationFlag(t *testing.T) {
	s := New("test")
	s.FlagSet().SetOutput(io.Discard)
	timeout := s.Duration("timeout", "t", 30*time.Second, "request timeout")

	cases := []struct {
		args []string
		want time.Duration
	}{
		{[]string{}, 30 * time.Second},
		{[]string{"--timeout", "5m"}, 5 * time.Minute},
		{[]string{"-t", "1h30m"}, 90 * time.Minute},
		{[]string{"--timeout=500ms"}, 500 * time.Millisecond},
	}
	for _, tc := range cases {
		t.Run(strings.Join(tc.args, " "), func(t *testing.T) {
			*timeout = 30 * time.Second
			if err := s.Parse(tc.args); err != nil {
				t.Fatalf("parse: %v", err)
			}
			if *timeout != tc.want {
				t.Errorf("timeout = %v, want %v", *timeout, tc.want)
			}
		})
	}
}

type commaList []string

func (c *commaList) String() string { return strings.Join(*c, ",") }
func (c *commaList) Set(v string) error {
	for p := range strings.SplitSeq(v, ",") {
		if p = strings.TrimSpace(p); p != "" {
			*c = append(*c, p)
		}
	}
	return nil
}

func TestVarFlagCustomType(t *testing.T) {
	s := New("test")
	s.FlagSet().SetOutput(io.Discard)
	var peers commaList
	s.Var(&peers, "peers", "p", "comma-separated peers")

	if err := s.Parse([]string{"-p", "alice, bob ,", "--peers", "carol"}); err != nil {
		t.Fatalf("parse: %v", err)
	}
	want := []string{"alice", "bob", "carol"}
	if len(peers) != len(want) {
		t.Fatalf("peers = %v, want %v", peers, want)
	}
	for i, v := range want {
		if peers[i] != v {
			t.Errorf("peers[%d] = %q, want %q", i, peers[i], v)
		}
	}
}

func TestVarShownInUsage(t *testing.T) {
	s := New("test")
	var buf bytes.Buffer
	s.FlagSet().SetOutput(&buf)
	var peers commaList
	s.Var(&peers, "peers", "p", "comma-separated peers")

	_ = s.Parse([]string{"-h"})
	out := buf.String()
	for _, frag := range []string{"-p, --peers", "comma-separated peers"} {
		if !strings.Contains(out, frag) {
			t.Errorf("usage output missing %q\n---\n%s", frag, out)
		}
	}
}

func TestUnknownFlagReturnsError(t *testing.T) {
	s := New("test")
	s.FlagSet().SetOutput(io.Discard)
	s.String("directory", "d", "", "")

	err := s.Parse([]string{"--nonexistent"})
	if err == nil {
		t.Fatal("expected error for unknown flag, got nil")
	}
	if errors.Is(err, flag.ErrHelp) {
		t.Errorf("unknown flag should not return ErrHelp: %v", err)
	}
}
