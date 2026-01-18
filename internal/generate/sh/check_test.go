//go:build !race

package main

import (
	"testing"

	"lesiw.io/command/internal/testcheck"
)

func TestCheck(t *testing.T) { testcheck.Run(t) }
