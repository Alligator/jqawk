package main

import (
	"os"

	cli "github.com/alligator/jqawk/cli"
)

var version = "0.5.9"

func main() {
	os.Exit(cli.Run(version))
}
