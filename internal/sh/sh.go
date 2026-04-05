package sh

import (
	"cmp"
	"fmt"
	"regexp"
	"slices"
	"strings"
)

var unsafe = regexp.MustCompile(`[^\w@%+=:,./-]`)

type Stringer string

func (s Stringer) String() string {
	return string(s)
}

var _ fmt.Stringer = Stringer("")

// Quote quotes a string for safe use in shell commands.
func Quote(s string) string {
	if s == "" {
		return `''`
	}
	if !unsafe.MatchString(s) {
		return s
	}
	var out strings.Builder
	out.Grow(len(s) + 2)
	out.WriteByte('\'')
	for i := range len(s) {
		if s[i] == '\'' {
			out.WriteString(`'\''`)
		} else {
			out.WriteByte(s[i])
		}
	}
	out.WriteByte('\'')
	return out.String()
}

// Join joins command arguments with proper shell quoting.
func Join(parts []string) string {
	quotedParts := make([]string, len(parts))
	for i, part := range parts {
		quotedParts[i] = Quote(part)
	}
	return strings.Join(quotedParts, " ")
}

// sortkeys returns the sorted keys of a map.
func sortkeys[K cmp.Ordered, V any](m map[K]V) []K {
	keys := make([]K, len(m))
	var i int
	for k := range m {
		keys[i] = k
		i++
	}
	slices.Sort(keys)
	return keys
}

// String returns a shell command string with environment variables and arguments.
func String(env map[string]string, arg ...string) Stringer {
	var ret strings.Builder
	for _, k := range sortkeys(env) {
		ret.WriteString(k + "=" + env[k] + " ")
	}
	ret.WriteString(Join(arg))
	return Stringer(ret.String())
}
