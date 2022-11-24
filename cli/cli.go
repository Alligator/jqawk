package cli

import (
	"encoding/json"
	"flag"
	"fmt"
	"go/ast"
	"io"
	"os"

	lang "github.com/alligator/jqawk/src"
)

func DebugAst(prog string, rootSelector string) {
	if len(rootSelector) > 0 {
		fmt.Println("root selector ast")
		rsLex := lang.NewLexer(rootSelector)
		rsParser := lang.NewParser(&rsLex)
		expr, err := rsParser.ParseExpression()
		if err != nil {
			panic(err)
		}
		ast.Print(nil, expr)
	}
	fmt.Println("program ast")
	lex := lang.NewLexer(prog)
	parser := lang.NewParser(&lex)
	program, err := parser.Parse()
	if err != nil {
		panic(err)
	}
	ast.Print(nil, program)
}

func Run() (exitCode int) {
	dbgAst := flag.Bool("dbg-ast", false, "print the AST")
	progFile := flag.String("f", "", "the program file to run")
	rootSelector := flag.String("r", "", "root selector")
	flag.Parse()

	var prog string
	var filePath string
	if len(*progFile) > 0 {
		filePath = flag.Arg(0)
		file, err := os.ReadFile(*progFile)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
		prog = string(file)
	} else {
		prog = flag.Arg(0)
		filePath = flag.Arg(1)
	}

	if *dbgAst {
		DebugAst(prog, *rootSelector)
		os.Exit(0)
	}

	var input io.Reader
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

	var rootValue interface{}
	if input != nil {
		b, err := io.ReadAll(input)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}

		err = json.Unmarshal(b, &rootValue)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
	}

	var rootCell *lang.Cell
	if len(*rootSelector) > 0 {
		cell, err := lang.EvalExpression(*rootSelector, rootValue, os.Stdout)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
		rootCell = cell
	} else {
		rootCell = lang.NewCell(lang.NewValue(rootValue))
	}

	err := lang.EvalProgram(prog, rootCell, os.Stdout)
	if err != nil {
		switch tErr := err.(type) {
		case lang.SyntaxError:
			fmt.Fprintf(os.Stderr, "  %s\n", tErr.SrcLine)
			fmt.Fprintf(os.Stderr, "  %*s\n", tErr.Col+1, "^")
			fmt.Fprintf(os.Stderr, "syntax error on line %d: %s\n", tErr.Line, tErr.Message)
		case lang.RuntimeError:
			fmt.Fprintf(os.Stderr, "  %s\n", tErr.SrcLine)
			fmt.Fprintf(os.Stderr, "  %*s\n", tErr.Col+1, "^")
			fmt.Fprintf(os.Stderr, "runtime error on line %d: %s\n", tErr.Line, tErr.Message)
		default:
			fmt.Fprintln(os.Stderr, err)
		}
		return 1
	}

	return 0
}
