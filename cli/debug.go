package cli

import (
	"fmt"
	"go/ast"
	"io"
	"strings"

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
				lang.PrintError(err, dst)
				return
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
	dbg := func(prog string) (string, bool) {
		var sb strings.Builder
		lex := lang.NewLexer(prog)
		line := 1
		fmt.Fprint(&sb, "   1: ")
		for {
			tok, err := lex.Next()
			if err != nil {
				lang.PrintError(err, dst)
				return "", false
			}

			if tok.Tag == lang.Divide {
				tok, err = lex.Regex()
				if err != nil {
					lang.PrintError(err, dst)
					return "", false
				}
			}

			if tok.Len == 0 {
				fmt.Fprintf(&sb, "%s ", tok.Tag)
			} else {
				fmt.Fprintf(&sb, "%s(%#v) ", tok.Tag, lex.GetString(&tok))
			}

			if tok.Tag == lang.Newline {
				line++
				fmt.Fprintf(&sb, "\n%4d: ", line)
			} else if tok.Tag == lang.EOF {
				break
			}
		}
		return sb.String(), true
	}

	if len(rootSelectors) > 0 {
		for i, rootSelector := range rootSelectors {
			result, ok := dbg(rootSelector)
			if !ok {
				return
			}
			fmt.Fprintf(dst, "root selector %d tokens\n", i+1)
			fmt.Fprintln(dst, result)
			fmt.Fprint(dst, "\n")
		}
	}

	result, ok := dbg(prog)
	if !ok {
		return
	}
	fmt.Fprintln(dst, "program tokens")
	fmt.Fprintln(dst, result)
}
