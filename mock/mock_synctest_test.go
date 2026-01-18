//go:build go1.25

package mock_test

import (
	"context"
	"strings"
	"testing"
	"testing/synctest"

	"lesiw.io/command"
	"lesiw.io/command/mock"
)

func TestMachineConcurrentPipeline(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		m, ctx := new(mock.Machine), context.Background()
		m.Return(strings.NewReader("FILTERED\n"), "filter")

		var result strings.Builder
		if _, err := command.Copy(&result,
			strings.NewReader("input\n"),
			command.NewFilter(ctx, m, "filter"),
		); err != nil {
			t.Fatal(err)
		}

		if got, want := result.String(), "FILTERED\n"; got != want {
			t.Errorf("output = %q, want %q", got, want)
		}

		if got, want := len(m.Calls), 1; got != want {
			t.Fatalf("call count = %d, want %d", got, want)
		}

		if got, want := m.Calls[0].Args[0], "filter"; got != want {
			t.Errorf("cmd = %q, want %q", got, want)
		}

		if got, want := string(m.Calls[0].Got), "input\n"; got != want {
			t.Errorf("input = %q, want %q", got, want)
		}
	})
}
