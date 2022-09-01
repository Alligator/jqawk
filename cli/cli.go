package cli

import (
	"flag"
	"fmt"
	"io"
	"os"

	lang "github.com/alligator/jqawk/src"
)

func Run() (exitCode int) {
	flag.Parse()
	prog := flag.Arg(0)

	var input io.Reader
	filePath := flag.Arg(1)
	if filePath == "" {
		input = os.Stdin
	} else {
		file, err := os.Open(filePath)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
		defer file.Close()
		input = file
	}

	lex := lang.NewLexer(prog)
	parser := lang.NewParser(&lex)
	rules, err := parser.Parse()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}

	ev := lang.NewEvaluator(rules, &lex, os.Stdout, input)
	err = ev.Eval()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}

	return 0
}
