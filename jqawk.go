package main

import (
	"os"

	cli "github.com/alligator/jqawk/cli"
)

func main() {
	os.Exit(cli.Run())
}
