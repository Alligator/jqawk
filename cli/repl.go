package cli

import (
	"fmt"
	"io"
	"os"
	"strings"

	lang "github.com/alligator/jqawk/src"
	"github.com/chzyer/readline"
)

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

	fmt.Printf("jqawk %s (revision %s)\n", version, getCommit())

	for {
		line, err := rl.Readline()
		if err != nil {
			fmt.Fprintf(os.Stderr, "readline error: %s\n", err.Error())
			return 1
		}
		line = strings.TrimSpace(line)

		_, err = lang.EvalProgram(line, bufferedFiles, rootSelectors, os.Stdout, false)
		if err != nil {
			printError(err)
		}
		fmt.Println("")
	}

	return 0
}
