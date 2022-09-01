package cli

import (
	"flag"
	"fmt"
	"go/ast"
	"io"
	"os"

	lang "github.com/alligator/jqawk/src"
)

func Run() (exitCode int) {
	dbgAst := flag.Bool("dbg-ast", false, "print the AST")
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

	if *dbgAst {
		ast.Print(nil, rules)
	}

	ev := lang.NewEvaluator(rules, &lex, os.Stdout, input)
	err = ev.Eval()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}

	return 0
}
