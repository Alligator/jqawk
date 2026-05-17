package cli

import (
	"fmt"
	"io"
	"strings"

	lang "github.com/alligator/jqawk/src"
	"github.com/ergochat/readline"
)

func printHelp(mode string, dst io.Writer) {
	fmt.Fprintln(dst, `commands
  :h :help   print this message
  :q :quit   quit
  :mode      switch mode`)
	fmt.Fprintln(dst)
	printMode(mode, dst)
}

func printModeHelp(mode string, dst io.Writer) {
	fmt.Fprintln(dst, `Modes control how each line is interpreted
Availiable modes are:

statement
  Each line is run once with $ set to the root value
  Example: print $[0]

program
  Each line is run as a full jqawk program
  Example: { print $.name }

usage
  :mode                      show this message
  :mode statement|program    switch mode`)
	fmt.Fprintln(dst)
	printMode(mode, dst)
}

func printMode(mode string, dst io.Writer) {
	switch mode {
	case "statement":
		fmt.Fprintln(dst, "current mode: statement (run once with $ = root)")
	case "program":
		fmt.Fprintln(dst, "current mode: program (run as full program)")
	}
}

func RunRepl(version string, files []lang.InputFile, rootSelectors []string, stdin io.Reader, stdout, stderr io.Writer) int {
	rl, err := readline.NewFromConfig(&readline.Config{
		Prompt: "> ",
		Stdin: stdin,
		Stdout: stdout,
		Stderr: stderr,
	})
	if err != nil {
		fmt.Fprintf(stderr, "error starting REPL: %s\n", err.Error())
		return 1
	}
	defer rl.Close()

	// convert each streaming input file into a buffered input file
	bufferedFiles := make([]lang.InputFile, len(files))
	for i, file := range files {
		if sif, ok := file.(*lang.StreamingInputFile); ok {
			if file.Name() == "<stdin>" {
				fmt.Fprintln(stderr, "cannot read from stdin in interactive mode")
				return 1
			}

			bytes, err := io.ReadAll(sif.NewReader())
			if err != nil {
				fmt.Fprintf(stderr, "error opening file: %s\n", err.Error())
				return 1
			}
			bufferedFiles[i] = lang.NewBufferedInputFile(file.Name(), bytes)
		} else {
			bufferedFiles[i] = file
		}
	}

	mode := "statement"
	fmt.Fprintf(stdout, "jqawk %s (revision %s)\nrun :help for help\n", version, getCommit())

	ev := lang.NewEmptyEvaluator(stdout)

	evalStmtAndMaybePrint := func(stmt lang.Statement) error {
		if s, ok := stmt.(*lang.StatementExpr); ok {
			val, err := ev.EvalExpr(s.Expr)
			if err != nil {
				return err
			}
			fmt.Fprintln(stdout, val.Value.PrettyString(false))
			return nil
		}

		return ev.EvalStatement(stmt)
	}

	for {
		line, err := rl.Readline()
		if err != nil {
			fmt.Fprintf(stderr, "readline error: %s\n", err.Error())
			return 1
		}
		line = strings.TrimSpace(line)

		if len(line) == 0 {
			continue
		}

		if strings.HasPrefix(line, ":") {
			parts := strings.Split(line, " ")
			switch parts[0] {
			case ":h", ":help":
				printHelp(mode, stdout)
			case ":q", ":quit":
				return 0
			case ":mode":
				if len(parts) != 2 {
					printModeHelp(mode, stdout)
					continue
				}
				switch parts[1] {
				case "program", "statement":
					mode = parts[1]
					printMode(mode, stdout)
				default:
					fmt.Fprintf(stdout, "unknown mode %s\n", parts[1])
				}
			default:
				fmt.Fprintf(stdout, "unknown command %s\n", parts[0])
			}
			continue
		}

		switch mode {
		case "statement":
			lex := lang.NewLexer(line)
			parser := lang.NewParser(&lex)
			stmt, err := parser.ParseStatement()
			if err != nil {
				lang.PrintError(err, stdout)
				continue
			}

			if len(bufferedFiles) == 0 {
				err = evalStmtAndMaybePrint(stmt)
			} else {
				err = ev.RunInBeginFileContext(bufferedFiles, rootSelectors, func() error {
					return evalStmtAndMaybePrint(stmt)
				})
			}

			if err != nil {
				lang.PrintError(err, stdout)
			}
		case "program":
			lex := lang.NewLexer(line)
			parser := lang.NewParser(&lex)
			prog, err := parser.Parse()
			if err != nil {
				lang.PrintError(err, stdout)
				continue
			}

			err = ev.RunProgram(prog, bufferedFiles, rootSelectors)

			if err != nil {
				lang.PrintError(err, stdout)
			}
		}
	}
}
