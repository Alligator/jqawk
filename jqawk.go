package main

import (
	"os"

	cli "github.com/alligator/jqawk/cli"
)

var version = "0.6.0"

func main() {
	os.Exit(cli.Run(version))
}
