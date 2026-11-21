//go:build windows

package main

import (
	"strings"
	"testing"
)

func TestParseCLIArgsAndHelp(t *testing.T) {
	opts := parseCLIArgs([]string{"--debug", "--reset-stats"})
	if !opts.debug || !opts.resetStats || opts.showHelp || opts.unknownArg != "" {
		t.Fatalf("unexpected parsed opts: %+v", opts)
	}

	optsClear := parseCLIArgs([]string{"--clear-stats"})
	if !optsClear.resetStats {
		t.Fatalf("expected --clear-stats alias to set resetStats=true")
	}

	optsHelp := parseCLIArgs([]string{"--help"})
	if !optsHelp.showHelp {
		t.Fatalf("expected --help to set showHelp=true")
	}

	optsBad := parseCLIArgs([]string{"--does-not-exist"})
	if optsBad.unknownArg != "--does-not-exist" {
		t.Fatalf("expected unknownArg captured, got %+v", optsBad)
	}

	help := FormatCLIHelp()
	for _, flag := range []string{"--debug", "--login", "--reset-stats", "--clear-stats", "--smoke-test", "--help"} {
		if !strings.Contains(help, flag) {
			t.Fatalf("expected CLI help text to document %s", flag)
		}
	}
}
