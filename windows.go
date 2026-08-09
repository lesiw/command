package command

import (
	"context"
	"fmt"
	"io"
	"strings"
)

// psScript joins PowerShell statements with ";" separator.
func psScript(statements ...string) string {
	return strings.Join(statements, ";")
}

func psRead(ctx context.Context, m Machine, script string, a ...any) (string, error) {
	return Read(ctx, m, "powershell",
		"-NonInteractive",
		"-InputFormat", "None",
		"-Command", fmt.Sprintf(script, a...),
	)
}

func psDo(ctx context.Context, m Machine, script string, a ...any) error {
	return Do(ctx, m, "powershell",
		"-NonInteractive",
		"-InputFormat", "None",
		"-Command", fmt.Sprintf(script, a...),
	)
}

// psReader returns an io.ReadCloser for PowerShell commands that only
// read output. Uses -InputFormat None since stdin is not needed.
func psReader(ctx context.Context, m Machine, script string, a ...any) io.ReadCloser {
	cmd := fmt.Sprintf(script, a...)
	return NewReader(ctx, m, "powershell",
		"-NonInteractive",
		"-InputFormat", "None",
		"-Command", cmd,
	)
}

// psWriter returns an io.WriteCloser for PowerShell commands that
// accept stdin input. Does NOT use -InputFormat None so that $input
// and stdin streams work correctly.
func psWriter(ctx context.Context, m Machine, script string, a ...any) io.WriteCloser {
	cmd := fmt.Sprintf(script, a...)
	return NewWriter(ctx, m, "powershell", "-NonInteractive", "-Command", cmd)
}

func dosReader(ctx context.Context, m Machine, args ...string) io.ReadCloser {
	return crlfReader(NewReader(ctx, m, args...))
}

func dosWriter(ctx context.Context, m Machine, args ...string) io.WriteCloser {
	return NewWriter(ctx, m, args...)
}
