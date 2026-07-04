package command

import (
	"context"
	"strings"
)

// OS detects the operating system of the given machine by probing it with
// various commands. It returns normalized GOOS values: "linux", "darwin",
// "freebsd", "openbsd", "netbsd", "dragonfly", "windows", or "unknown".
//
// If m implements OSMachine, OS() calls m.OS(ctx) and uses the returned
// value. If m.OS(ctx) returns an empty string, OS falls back to probing
// with commands.
//
// OS automatically pierces through Shell layers by trying each probe command
// first on the given machine, then unshelling and retrying if the command
// is not found. This allows Shell handlers to override probe commands for
// testing while still falling back to the underlying system.
func OS(ctx context.Context, m Machine) string {
	if osm, ok := m.(OSMachine); ok {
		if os := osm.OS(ctx); os != "" {
			return os
		}
	}
	return detectOS(ctx, m)
}

func detectOS(ctx context.Context, m Machine) string {
	out, err := probeRead(ctx, m, "uname", "-s")
	if err == nil {
		os := strings.TrimSpace(strings.ToLower(out))
		if strings.Contains(os, "msys_nt") {
			return "windows"
		}
		return os
	}

	out, err = probeRead(ctx, m, "cmd", "/c", "ver")
	if err == nil && strings.Contains(strings.ToLower(out), "windows") {
		return "windows"
	}

	// Windows OpenSSH servers can mangle the quoting of cmd /c
	// invocations, so probe again through PowerShell, which also
	// covers hosts whose default shell is PowerShell itself.
	out, err = probeRead(ctx, m,
		"powershell", "-NoProfile", "-NonInteractive", "-Command", "$env:OS",
	)
	if err == nil && strings.Contains(strings.ToLower(out), "windows") {
		return "windows"
	}

	return "unknown"
}

// Arch detects the architecture of the given machine by probing it with
// various commands. It returns normalized GOARCH values: "amd64", "arm64",
// "386", "arm", or "unknown".
//
// If m implements ArchMachine, Arch() calls m.Arch(ctx) and uses the returned
// value. If m.Arch(ctx) returns an empty string, Arch falls back to probing
// with commands.
//
// Arch automatically pierces through Shell layers by trying each probe command
// first on the given machine, then unshelling and retrying if the command
// is not found. This allows Shell handlers to override probe commands for
// testing while still falling back to the underlying system.
func Arch(ctx context.Context, m Machine) string {
	if archm, ok := m.(ArchMachine); ok {
		if arch := archm.Arch(ctx); arch != "" {
			return arch
		}
	}
	return detectArch(ctx, m)
}

func detectArch(ctx context.Context, m Machine) string {
	// On Darwin/macOS only, use uname -v to detect architecture from kernel.
	//
	// We cannot rely on uname -m because of how macOS handles universal
	// binaries and Rosetta translation. When a process runs under Rosetta
	// (x86_64 translation), child processes inherit the "translated" flag
	// even if the child binary is native arm64. This causes macOS to
	// preferentially run the x86_64 slice of universal binaries like uname.
	//
	// For example, when an arm64 Go binary is spawned from an x86_64 shell,
	// the Go process inherits the translated flag. When it runs uname -m,
	// macOS executes the x86_64 slice, returning "x86_64" even though the
	// hardware is arm64.
	//
	// The kernel version string (from uname -v) always reflects the actual
	// hardware architecture (e.g., "RELEASE_ARM64_T8132"), regardless of
	// which process architecture is running or which binary slice executes.
	//
	// Only use this method on Darwin to avoid false positives on other OSes.
	if OS(ctx, m) == "darwin" {
		output, err := probeRead(ctx, m, "uname", "-v")
		if err == nil {
			version := strings.ToUpper(output)
			if strings.Contains(version, "ARM64") {
				return "arm64"
			}
			if strings.Contains(version, "X86_64") {
				return "amd64"
			}
		}
	}

	out, err := probeRead(ctx, m, "uname", "-m")
	if err == nil {
		return mapArchitecture(strings.TrimSpace(out))
	}

	out, err = probeRead(ctx, m,
		"cmd", "/c", "echo %PROCESSOR_ARCHITECTURE%",
	)
	if err == nil {
		// Windows OpenSSH servers can mangle this invocation's quoting,
		// so only trust the probe when it maps to a known architecture.
		if arch := mapArchitecture(strings.TrimSpace(out)); arch != "unknown" {
			return arch
		}
	}

	out, err = probeRead(ctx, m,
		"powershell", "-NoProfile", "-NonInteractive", "-Command",
		"$env:PROCESSOR_ARCHITECTURE",
	)
	if err == nil {
		return mapArchitecture(strings.TrimSpace(out))
	}

	return "unknown"
}

func mapArchitecture(arch string) string {
	switch strings.ToLower(arch) {
	case "x86_64", "x86-64", "x64", "amd64":
		return "amd64"
	case "aarch64", "arm64":
		return "arm64"
	case "i386", "i486", "i586", "i686", "x86":
		return "386"
	case "armv7l", "armv6l", "arm":
		return "arm"
	default:
		return "unknown"
	}
}
