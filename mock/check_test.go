//go:build !remote && !race

package mock

import (
	"testing"

	"lesiw.io/command/internal/testcheck"
)

func TestCheck(t *testing.T) { testcheck.Run(t) }
