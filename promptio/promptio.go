// Package promptio provides context-aware interactive terminal input.
//
// Stdin reads are not cancellable, so helpers run the read in a goroutine and
// return early on ctx cancellation; the goroutine leaks until process exit.
package promptio

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"

	"golang.org/x/term"
)

type result struct {
	s   string
	err error
}

func ReadLine(ctx context.Context, msg string) (string, error) {
	return readLine(ctx, os.Stdout, os.Stdin, msg)
}

func readLine(ctx context.Context, w io.Writer, r io.Reader, msg string) (string, error) {
	fmt.Fprint(w, msg)

	ch := make(chan result, 1)
	go func() {
		s, err := readLineBytes(r)
		ch <- result{s: s, err: err}
	}()

	return awaitResult(ctx, w, ch)
}

func readLineBytes(r io.Reader) (string, error) {
	var buf []byte
	b := make([]byte, 1)
	for {
		n, err := r.Read(b)
		if n > 0 {
			if b[0] == '\n' {
				return strings.TrimSpace(string(buf)), nil
			}
			buf = append(buf, b[0])
		}
		if err != nil {
			if err == io.EOF && len(buf) > 0 {
				return strings.TrimSpace(string(buf)), nil
			}
			return "", err
		}
	}
}

func ReadPassword(ctx context.Context, msg string) (string, error) {
	fmt.Fprint(os.Stdout, msg)

	ch := make(chan result, 1)
	go func() {
		b, err := term.ReadPassword(int(os.Stdin.Fd()))
		fmt.Fprintln(os.Stdout)
		ch <- result{s: string(b), err: err}
	}()

	return awaitResult(ctx, os.Stdout, ch)
}

func awaitResult(ctx context.Context, w io.Writer, ch <-chan result) (string, error) {
	select {
	case <-ctx.Done():
		fmt.Fprintln(w)
		return "", ctx.Err()
	case res := <-ch:
		return res.s, res.err
	}
}
