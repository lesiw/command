package ctr

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"

	"lesiw.io/command"
	"lesiw.io/command/mock"
)

func TestMachineFindsDocker(t *testing.T) {
	m := new(mock.Machine)
	m.Return(strings.NewReader("abc123\n"), "docker", "container", "run")
	ctr := Machine(m, "alpine")

	if _, err := command.Read(t.Context(), ctr, "true"); err != nil {
		t.Fatalf("command.Call error: %v", err)
	}

	mockCalls := []mock.Call{
		{Args: []string{"docker", "--version"}},
		{Args: []string{"docker", "container", "run",
			"--rm", "-d", "-i", "alpine", "cat"}},
		{Args: []string{"docker", "container", "exec",
			"-i", "abc123", "true"}},
	}
	if got, want := mock.Calls(m), mockCalls; !cmp.Equal(got, want) {
		t.Errorf("mock calls (-want +got):\n%s", cmp.Diff(want, got))
	}
}

func TestMachineFindsPodman(t *testing.T) {
	m := new(mock.Machine)
	m.Return(command.Fail(&command.Error{
		Err: fmt.Errorf("command not found: docker"),
	}), "docker", "--version")
	m.Return(strings.NewReader("xyz789\n"), "podman", "container", "run")
	ctr := Machine(m, "alpine")

	if _, err := command.Read(t.Context(), ctr, "true"); err != nil {
		t.Fatalf("command.Call error: %v", err)
	}

	mockCalls := []mock.Call{
		{Args: []string{"docker", "--version"}},
		{Args: []string{"podman", "--version"}},
		{Args: []string{"podman", "container", "run",
			"--rm", "-d", "-i", "alpine", "cat"}},
		{Args: []string{"podman", "container", "exec",
			"-i", "xyz789", "true"}},
	}
	if got, want := mock.Calls(m), mockCalls; !cmp.Equal(got, want) {
		t.Errorf("mock calls (-want +got):\n%s", cmp.Diff(want, got))
	}
}

func TestMachineShutdown(t *testing.T) {
	m := new(mock.Machine)
	m.Return(strings.NewReader("abc123\n"), "docker", "container", "run")
	ctr := Machine(m, "alpine")

	if _, err := command.Read(t.Context(), ctr, "true"); err != nil {
		t.Fatalf("command.Call error: %v", err)
	}
	if err := command.Shutdown(t.Context(), ctr); err != nil {
		t.Fatalf("command.Shutdown error: %v", err)
	}

	calls := mock.Calls(m)
	lastCall := mock.Call{
		Args: []string{"docker", "container", "rm", "-f", "abc123"},
	}
	if got, want := calls[len(calls)-1], lastCall; !cmp.Equal(got, want) {
		t.Errorf("mock calls (-want +got):\n%s", cmp.Diff(want, got))
	}
}

func TestMachineNoContainerCLI(t *testing.T) {
	m := new(mock.Machine)
	for _, cli := range clis {
		m.Return(command.Fail(&command.Error{
			Err: fmt.Errorf("command not found: %s", cli),
		}), cli...)
	}

	ctr := Machine(m, "alpine")

	_, err := command.Read(t.Context(), ctr, "true")
	if err == nil {
		t.Fatalf("command.Call error: got nil, want non-nil")
	}
	want := "no container CLI found"
	if !strings.Contains(err.Error(), want) {
		t.Errorf("command.Call error: got %v, want %q", err, want)
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
