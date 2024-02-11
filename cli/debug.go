package cli

import (
	"fmt"
	"go/ast"

	lang "github.com/alligator/jqawk/src"
)

func debugAst(prog string, rootSelector string) {
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

func debugLex(prog string, rootSelector string) {
	dbg := func(prog string) {
		lex := lang.NewLexer(prog)
		line := 1
		fmt.Print("   1: ")
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
				fmt.Printf("%s ", tok.Tag)
			} else {
				fmt.Printf("%s(%#v) ", tok.Tag, lex.GetString(&tok))
			}

			if tok.Tag == lang.Newline {
				line++
				fmt.Printf("\n%4d: ", line)
			} else if tok.Tag == lang.EOF {
				break
			}
		}
	}
	if len(rootSelector) > 0 {
		fmt.Println("root selector tokens")
		dbg(rootSelector)
		fmt.Print("\n")
	}
	fmt.Println("program tokens")
	dbg(prog)
}
