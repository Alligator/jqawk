package cli

import (
	"fmt"
	"io"
	"os"
	"strings"

	lang "github.com/alligator/jqawk/src"
	"github.com/chzyer/readline"
)

func printHelp(mode string) {
	fmt.Println(`commands
  :h :help   print this message
  :q :quit   quit
  :mode      switch mode`)
	fmt.Println()
	printMode(mode)
}

func printModeHelp(mode string) {
	fmt.Println(`Modes control how each line is interpreted
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
	fmt.Println()
	printMode(mode)
}

func printMode(mode string) {
	switch mode {
	case "statement":
		fmt.Println("current mode: statement (run once with $ = root)")
	case "program":
		fmt.Println("current mode: program (run as full program)")
	}
}

func readCommand(line string) {
}

func RunRepl(version string, files []lang.InputFile, rootSelectors []string) int {
	rl, err := readline.New("> ")
	if err != nil {
		fmt.Fprintf(os.Stderr, "error starting REPL: %s\n", err.Error())
		return 1
	}
	defer rl.Close()

	// convert each streaming input file into a buffered input file
	bufferedFiles := make([]lang.InputFile, len(files))
	for i, file := range files {
		if sif, ok := file.(*lang.StreamingInputFile); ok {
			if file.Name() == "<stdin>" {
				fmt.Fprintln(os.Stderr, "cannot read from stdin in interactive mode")
				return 1
			}

			bytes, err := io.ReadAll(sif.NewReader())
			if err != nil {
				fmt.Fprintf(os.Stderr, "error opening file: %s\n", err.Error())
				return 1
			}
			bufferedFiles[i] = lang.NewBufferedInputFile(file.Name(), bytes)
		} else {
			bufferedFiles[i] = file
		}
	}

	mode := "statement"
	fmt.Printf("jqawk %s (revision %s)\nrun :help for help\n", version, getCommit())

	ev := lang.NewEmptyEvaluator(os.Stdout)

	for {
		line, err := rl.Readline()
		if err != nil {
			fmt.Fprintf(os.Stderr, "readline error: %s\n", err.Error())
			return 1
		}
		line = strings.TrimSpace(line)

		if strings.HasPrefix(line, ":") {
			parts := strings.Split(line, " ")
			switch parts[0] {
			case ":h", ":help":
				printHelp(mode)
			case ":q", ":quit":
				return 0
			case ":mode":
				if len(parts) != 2 {
					printModeHelp(mode)
					continue
				}
				switch parts[1] {
				case "program", "statement":
					mode = parts[1]
					printMode(mode)
				default:
					fmt.Printf("unknown mode %s\n", parts[1])
				}
			default:
				fmt.Printf("unknown command %s\n", parts[0])
			}
			continue
		}

		switch mode {
		case "statement":
			err = ev.RunInBeginFileContext(bufferedFiles, rootSelectors, func() error {
				lex := lang.NewLexer(line)
				parser := lang.NewParser(&lex)
				stmt, err := parser.ParseStatement()
				if err != nil {
					return err
				}

				return ev.EvalStatement(stmt)
			})

			if err != nil {
				lang.PrintError(err)
			}
		case "program":
			lex := lang.NewLexer(line)
			parser := lang.NewParser(&lex)
			prog, err := parser.Parse()
			if err != nil {
				lang.PrintError(err)
				continue
			}

			err = ev.RunProgram(prog, bufferedFiles, rootSelectors)

			if err != nil {
				lang.PrintError(err)
			}
		}
	}
}
