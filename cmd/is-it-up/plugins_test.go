package main

import (
	"slices"
	"testing"

	"github.com/LanceSandino/is-it-up/v2/pkg/checks"
)

func TestMainActivatesDNSPlugin(t *testing.T) {
	if !slices.Contains(checks.SupportedCheckTypes(), "dns") {
		t.Fatal("the CLI should activate the dns plugin")
	}
}
