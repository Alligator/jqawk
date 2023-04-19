package main

import (
	"os"

	cli "github.com/alligator/jqawk/cli"
)

var version = "0.5.2"

func main() {
	os.Exit(cli.Run(version))
}
