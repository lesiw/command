package ctr

import (
	"errors"
	"fmt"
	"slices"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"

	"lesiw.io/command"
	"lesiw.io/command/mock"
)

func callSuffix(calls []mock.Call, suffix []string) bool {
	for _, call := range calls {
		args := call.Args
		if len(args) >= len(suffix) {
			if slices.Equal(args[len(args)-len(suffix):], suffix) {
				return true
			}
		}
	}
	return false
}

func TestCtl(t *testing.T) {
	m := new(mock.Machine)
	ctl := Ctl(m)

	if err := command.Do(t.Context(), ctl, "true"); err != nil {
		t.Fatalf("command.Do error: %v", err)
	}

	if got, want := mock.Calls(m), []string{"true"}; !callSuffix(got, want) {
		t.Errorf("calls:\n%+v\nwant call ending with: %v", got, want)
	}
}

func TestCtlFindsDocker(t *testing.T) {
	m := new(mock.Machine)
	ctl := Ctl(m)

	err := command.Do(t.Context(), ctl, "container", "run", "alpine")
	if err != nil {
		t.Fatalf("command.Do error: %v", err)
	}

	mockCalls := []mock.Call{
		{Args: []string{"docker", "--version"}},
		{Args: []string{"docker", "container", "run", "alpine"}},
	}
	if got, want := mock.Calls(m), mockCalls; !cmp.Equal(got, want) {
		t.Errorf("mock calls (-want +got):\n%s", cmp.Diff(want, got))
	}
}

func TestCtlFindsPodman(t *testing.T) {
	m := new(mock.Machine)
	m.Return(command.Fail(&command.Error{
		Err: fmt.Errorf("command not found: docker"),
	}), "docker", "--version")
	ctl := Ctl(m)

	err := command.Do(t.Context(), ctl, "container", "run", "alpine")
	if err != nil {
		t.Fatalf("command.Do error: %v", err)
	}

	mockCalls := []mock.Call{
		{Args: []string{"docker", "--version"}},
		{Args: []string{"podman", "--version"}},
		{Args: []string{"podman", "container", "run", "alpine"}},
	}
	if got, want := mock.Calls(m), mockCalls; !cmp.Equal(got, want) {
		t.Errorf("mock calls (-want +got):\n%s", cmp.Diff(want, got))
	}
}

func TestCtlNoContainerCLI(t *testing.T) {
	m := new(mock.Machine)
	for _, cli := range clis {
		m.Return(command.Fail(&command.Error{
			Err: fmt.Errorf("command not found: %s", cli),
		}), cli...)
	}
	ctl := Ctl(m)

	err := command.Do(t.Context(), ctl, "container", "run", "alpine")
	if err == nil {
		t.Fatalf("command.Do error: got nil, want non-nil")
	}
	want := "no container CLI found"
	if !strings.Contains(err.Error(), want) {
		t.Errorf("command.Do error: got %v, want %q", err, want)
	}
}

func TestMachineShutdown(t *testing.T) {
	m := new(mock.Machine)
	m.Return(strings.NewReader("abc123"), "docker", "container", "run")
	ctr := Machine(m, "alpine")

	if err := command.Do(t.Context(), ctr, "true"); err != nil {
		t.Fatalf("command.Do error: %v", err)
	}
	if err := command.Shutdown(t.Context(), ctr); err != nil {
		t.Fatalf("command.Shutdown error: %v", err)
	}

	calls := mock.Calls(m)
	want := []string{"container", "rm", "-f", "abc123"}
	if !callSuffix(calls, want) {
		t.Errorf("calls:\n%+v\nwant call ending with: %v", calls, want)
	}
}

func TestMachineShutdownBeforeInit(t *testing.T) {
	m := new(mock.Machine)
	ctr := Machine(m, "alpine")

	if err := command.Shutdown(t.Context(), ctr); err != nil {
		t.Fatalf("command.Shutdown error: %v", err)
	}

	err := command.Do(t.Context(), ctr, "true")
	if !errors.Is(err, errShutdown) {
		t.Errorf("command.Do after Shutdown: got %v, want errShutdown", err)
	}
}

func TestMachineCommandAfterShutdown(t *testing.T) {
	m := new(mock.Machine)
	m.Return(strings.NewReader("abc123"), "docker", "container", "run")
	ctr := Machine(m, "alpine")

	if err := command.Do(t.Context(), ctr, "true"); err != nil {
		t.Fatalf("command.Do error: %v", err)
	}
	if err := command.Shutdown(t.Context(), ctr); err != nil {
		t.Fatalf("command.Shutdown error: %v", err)
	}
	err := command.Do(t.Context(), ctr, "true")
	if !errors.Is(err, errShutdown) {
		t.Errorf("command.Do after Shutdown: got %v, want errShutdown", err)
	}
}
