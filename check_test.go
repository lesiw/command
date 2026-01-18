//go:build !remote && !race

package command

import (
	"testing"

	"lesiw.io/command/internal/testcheck"
)

func TestCheck(t *testing.T) { testcheck.Run(t) }
