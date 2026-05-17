package cli

import (
	"fmt"
	"go/ast"
	"io"

	lang "github.com/alligator/jqawk/src"
)

func debugAst(prog string, rootSelectors []string, dst io.Writer) {
	if len(rootSelectors) > 0 {
		for i, rootSelector := range rootSelectors {
			fmt.Fprintf(dst, "root selector %d ast\n", i)
			rsLex := lang.NewLexer(rootSelector)
			rsParser := lang.NewParser(&rsLex)
			expr, err := rsParser.ParseExpression()
			if err != nil {
				panic(err)
			}
			ast.Fprint(dst, nil, expr, ast.NotNilFilter)
		}
	}
	fmt.Fprintln(dst, "program ast")
	lex := lang.NewLexer(prog)
	parser := lang.NewParser(&lex)
	program, err := parser.Parse()
	if err != nil {
		lang.PrintError(err, dst)
		return
	}
	ast.Fprint(dst, nil, program, ast.NotNilFilter)
}

func debugLex(prog string, rootSelectors []string, dst io.Writer) {
	dbg := func(prog string) {
		lex := lang.NewLexer(prog)
		line := 1
		fmt.Fprint(dst, "   1: ")
		for {
			tok, err := lex.Next()
			if err != nil {
				panic(err)
			}

			if tok.Tag == lang.Divide {
				tok, err = lex.Regex()
				if err != nil {
					panic(err)
				}
			}

			if tok.Len == 0 {
				fmt.Fprintf(dst, "%s ", tok.Tag)
			} else {
				fmt.Fprintf(dst, "%s(%#v) ", tok.Tag, lex.GetString(&tok))
			}

			if tok.Tag == lang.Newline {
				line++
				fmt.Fprintf(dst, "\n%4d: ", line)
			} else if tok.Tag == lang.EOF {
				break
			}
		}
	}

	if len(rootSelectors) > 0 {
		for i, rootSelector := range rootSelectors {
			fmt.Fprintf(dst, "root selector %d tokens\n", i)
			dbg(rootSelector)
			fmt.Fprint(dst, "\n")
		}
	}
	fmt.Fprintln(dst, "program tokens")
	dbg(prog)
}
