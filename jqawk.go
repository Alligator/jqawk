package main

import (
	"os"

	"github.com/alligator/jqawk/cli"
	"github.com/mattn/go-isatty"
)

var version = "0.6.8"

func main() {
	tty := false
	if isatty.IsTerminal(os.Stdin.Fd()) {
		tty = true
	}
	os.Exit(cli.Run(version, os.Args[1:], os.Stdin, os.Stdout, os.Stderr, tty))
}
