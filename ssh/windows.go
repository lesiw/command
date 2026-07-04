package ssh

import (
	"context"
	"encoding/base64"
	"fmt"
	"regexp"
	"strings"
	"unicode/utf16"

	"lesiw.io/command"
	"lesiw.io/fs"
)

// windowsCommand encodes the remote invocation as a PowerShell script
// passed via -EncodedCommand. Windows OpenSSH hands the joined
// arguments to the remote account's default shell, which is
// configurable and unknown to the client, so no in-band quoting
// reliably survives it; the base64 channel contains no characters for
// any shell to interpret.
func (sm *machine) windowsCommand(
	ctx context.Context, args ...string,
) command.Buffer {
	var script strings.Builder
	// Stop makes failures terminate the script: without it, a failed
	// Set-Location would leave the command running in the wrong
	// directory, and a not-found command would leave $LASTEXITCODE
	// null, turning the final exit into a success.
	script.WriteString("$ErrorActionPreference = 'Stop'\n")
	for k, v := range command.Envs(ctx) {
		fmt.Fprintf(&script, "$env:%s = %s\n", k, psQuote(v))
	}
	ctx = command.WithoutEnv(ctx)
	if dir := fs.WorkDir(ctx); dir != "" {
		fmt.Fprintf(&script, "Set-Location %s\n", psQuote(dir))
		ctx = fs.WithoutWorkDir(ctx)
	}
	script.WriteString("&")
	for _, arg := range args {
		script.WriteByte(' ')
		script.WriteString(psQuote(psNativeEscape(arg)))
	}
	script.WriteString("\nexit $LASTEXITCODE\n")

	fullArgs := append(append([]string(nil), sm.args...),
		"powershell.exe", "-NoProfile", "-NonInteractive",
		"-EncodedCommand", psEncode(script.String()),
	)
	return sm.m.Command(ctx, fullArgs...)
}

// psQuote quotes a string as a PowerShell single-quoted literal,
// in which the only special character is the quote itself.
func psQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "''") + "'"
}

var psQuoteRun = regexp.MustCompile(`(\\*)"`)

// psNativeEscape prepares an argument for Windows PowerShell's native
// command invocation. PowerShell 5 forwards arguments to native
// programs without escaping embedded double quotes, so quotes are
// escaped here, doubling any backslash run that precedes them.
// A trailing backslash run is doubled when the argument contains
// whitespace, since PowerShell wraps such arguments in quotes.
func psNativeEscape(s string) string {
	e := psQuoteRun.ReplaceAllString(s, `$1$1\"`)
	if strings.ContainsAny(e, " \t") {
		trail := len(e) - len(strings.TrimRight(e, `\`))
		e += strings.Repeat(`\`, trail)
	}
	return e
}

// psEncode encodes a script for PowerShell's -EncodedCommand flag,
// which expects base64-encoded UTF-16LE.
func psEncode(script string) string {
	codes := utf16.Encode([]rune(script))
	b := make([]byte, len(codes)*2)
	for i, c := range codes {
		b[i*2] = byte(c)
		b[i*2+1] = byte(c >> 8)
	}
	return base64.StdEncoding.EncodeToString(b)
}
