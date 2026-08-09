package command_test

import (
	"testing"

	"lesiw.io/command"
	"lesiw.io/command/mem"
)

func TestEnvQueryMachine(t *testing.T) {
	var (
		m   = mem.Machine()
		ctx = command.WithEnv(t.Context(), map[string]string{
			"TEST_VAR": "from_machine",
		})
		got = command.Env(ctx, m, "TEST_VAR")
	)
	if want := "from_machine"; got != want {
		t.Errorf("Env(TEST_VAR) = %q, want %q", got, want)
	}
	got = command.Env(ctx, m, "NONEXISTENT")
	if want := ""; got != want {
		t.Errorf("Env(NONEXISTENT) = %q, want %q", got, want)
	}
}

func TestEnvContextOverridesMachine(t *testing.T) {
	var (
		m   = mem.Machine()
		ctx = command.WithEnv(t.Context(), map[string]string{
			"HOME": "/home/mem",
		})
	)

	if got, want := command.Env(ctx, m, "HOME"), "/home/mem"; got != want {
		t.Errorf("Env(HOME) = %q, want %q", got, want)
	}
}

func TestWithoutEnv(t *testing.T) {
	ctx := command.WithEnv(t.Context(), map[string]string{
		"FOO": "bar",
		"BAZ": "qux",
	})
	if got, want := len(command.Envs(ctx)), 2; got != want {
		t.Fatalf("command.Envs() got %d vars, want %d", got, want)
	}
	if got := command.Envs(command.WithoutEnv(ctx)); got != nil {
		t.Errorf("command.Envs() = %v, want nil", got)
	}
}
