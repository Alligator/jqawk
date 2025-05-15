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

	// read each file into memory and give them a reader on the string
	stringReaders := make([]*strings.Reader, 0)
	for _, file := range files {
		bytes, err := io.ReadAll(file.Reader)
		if err != nil {
			return 1
		}
		reader := strings.NewReader(string(bytes))
		file.Reader = reader
		stringReaders = append(stringReaders, reader)
	}

	fmt.Printf("jqawk %s (revision %s)\n", version, getCommit())

	for {
		line, err := rl.Readline()
		if err != nil {
			break
		}
		line = strings.TrimSpace(line)

		_, err = lang.EvalProgram(line, files, rootSelectors, os.Stdout, false)
		if err != nil {
			printError(err)
		}
		fmt.Println("")

		// reset all the file readers
		for _, reader := range stringReaders {
			reader.Seek(0, 0)
		}
	}

	return 0
}
