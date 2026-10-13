package main

import (
	"io"
	"testing"
)

func TestUsage(t *testing.T) {
	for _, args := range [][]string{nil, {"other"}, {"grant"}, {"grant", "--email", "u@example.com", "extra"}} {
		if err := run(args, io.Discard); err == nil {
			t.Fatal("invalid command accepted", args)
		}
	}
}
