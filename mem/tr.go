package mem

import (
	"context"
	"io"
	"strings"
	"sync"

	"lesiw.io/command"
	"lesiw.io/command/internal/sh"
)

type trCmd struct {
	ctx      context.Context
	args     []string
	replacer *strings.Replacer
	once     sync.Once
	inpr     *io.PipeReader
	inpw     *io.PipeWriter
	outpr    *io.PipeReader
	outpw    *io.PipeWriter
}

func trCommand(ctx context.Context, args ...string) command.Buffer {
	set1, set2 := "", ""
	if len(args) >= 3 {
		set1, set2 = args[1], args[2]
	}

	// Build replacement pairs for strings.NewReplacer
	from := expandSet(set1)
	to := expandSet(set2)
	var pairs []string
	for i, r := range from {
		if i < len(to) {
			pairs = append(pairs, string(r), string(to[i]))
		} else if len(to) > 0 {
			// If set2 is shorter, repeat last character
			pairs = append(pairs, string(r), string(to[len(to)-1]))
		}
	}

	inpr, inpw := io.Pipe()
	outpr, outpw := io.Pipe()

	return &trCmd{
		ctx:      ctx,
		args:     args,
		replacer: strings.NewReplacer(pairs...),
		inpr:     inpr,
		inpw:     inpw,
		outpr:    outpr,
		outpw:    outpw,
	}
}

func (c *trCmd) init() {
	go func() {
		defer c.outpw.Close()
		buf := make([]byte, 4096)
		for {
			n, err := c.inpr.Read(buf)
			if n > 0 {
				translated := c.replacer.Replace(string(buf[:n]))
				if _, werr := c.outpw.Write([]byte(translated)); werr != nil {
					return
				}
			}
			if err != nil {
				return
			}
		}
	}()
}

func (c *trCmd) Read(p []byte) (int, error) {
	c.once.Do(c.init)
	return c.outpr.Read(p)
}

func (c *trCmd) Write(p []byte) (int, error) {
	c.once.Do(c.init)
	return c.inpw.Write(p)
}

func (c *trCmd) Close() error {
	return c.inpw.Close()
}

func (c *trCmd) String() string {
	return sh.String(command.Envs(c.ctx), c.args...).String()
}

// expandSet expands character ranges like a-z into individual characters.
func expandSet(s string) []rune {
	var (
		result []rune
		runes  = []rune(s)
	)

	for i := 0; i < len(runes); i++ {
		if i+2 < len(runes) && runes[i+1] == '-' {
			// Range detected
			var (
				start = runes[i]
				end   = runes[i+2]
			)

			// Handle both forward and backward ranges
			if start <= end {
				for r := start; r <= end; r++ {
					result = append(result, r)
				}
			} else {
				for r := start; r >= end; r-- {
					result = append(result, r)
				}
			}
			i += 2 // Skip the '-' and end character
		} else {
			result = append(result, runes[i])
		}
	}

	return result
}
