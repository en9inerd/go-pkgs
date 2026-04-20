package promptio

import (
	"bytes"
	"context"
	"errors"
	"io"
	"strings"
	"testing"
	"time"
)

func TestReadLine_Happy(t *testing.T) {
	var out bytes.Buffer
	got, err := readLine(context.Background(), &out, strings.NewReader("hello\n"), "prompt: ")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "hello" {
		t.Errorf("got %q, want %q", got, "hello")
	}
	if !strings.Contains(out.String(), "prompt: ") {
		t.Errorf("prompt not written: %q", out.String())
	}
}

func TestReadLine_TrimsWhitespace(t *testing.T) {
	got, err := readLine(context.Background(), io.Discard, strings.NewReader("  padded  \n"), "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "padded" {
		t.Errorf("got %q, want %q", got, "padded")
	}
}

func TestReadLine_EOF(t *testing.T) {
	_, err := readLine(context.Background(), io.Discard, strings.NewReader(""), "")
	if !errors.Is(err, io.EOF) {
		t.Errorf("want io.EOF, got %v", err)
	}
}

func TestReadLine_ReaderError(t *testing.T) {
	want := errors.New("boom")
	_, err := readLine(context.Background(), io.Discard, &errReader{err: want}, "")
	if !errors.Is(err, want) {
		t.Errorf("want %v, got %v", want, err)
	}
}

func TestReadLine_CtxCancelledBeforeCall(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	// Use a blocking reader so the goroutine never produces a result; only
	// ctx cancellation can unblock the select.
	_, err := readLine(ctx, io.Discard, blockingReader{}, "")
	if !errors.Is(err, context.Canceled) {
		t.Errorf("want context.Canceled, got %v", err)
	}
}

func TestReadLine_CtxCancelledMidRead(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())

	done := make(chan struct{})
	var gotErr error
	go func() {
		_, gotErr = readLine(ctx, io.Discard, blockingReader{}, "")
		close(done)
	}()

	time.Sleep(20 * time.Millisecond)
	cancel()

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("readLine did not return after ctx cancel")
	}
	if !errors.Is(gotErr, context.Canceled) {
		t.Errorf("want context.Canceled, got %v", gotErr)
	}
}

func TestReadLine_WritesNewlineOnCancel(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	var out bytes.Buffer
	_, _ = readLine(ctx, &out, blockingReader{}, "prompt: ")
	if !strings.HasSuffix(out.String(), "\n") {
		t.Errorf("expected trailing newline, got %q", out.String())
	}
}

func TestReadLine_SequentialCallsShareReader(t *testing.T) {
	r := strings.NewReader("first\nsecond\nthird\n")

	for _, want := range []string{"first", "second", "third"} {
		got, err := readLine(context.Background(), io.Discard, r, "")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got != want {
			t.Errorf("got %q, want %q", got, want)
		}
	}
}

func TestReadLine_EOFWithPartialLine(t *testing.T) {
	got, err := readLine(context.Background(), io.Discard, strings.NewReader("no-newline"), "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "no-newline" {
		t.Errorf("got %q, want %q", got, "no-newline")
	}
}

type errReader struct{ err error }

func (e *errReader) Read(_ []byte) (int, error) { return 0, e.err }

type blockingReader struct{}

func (blockingReader) Read(_ []byte) (int, error) {
	select {} // never returns
}
