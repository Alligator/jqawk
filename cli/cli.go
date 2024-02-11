package cli

import (
	"flag"
	"fmt"
	"os"
	"runtime/debug"
	"runtime/pprof"

	lang "github.com/alligator/jqawk/src"
	"github.com/mattn/go-isatty"
)

func getCommit() string {
	if info, ok := debug.ReadBuildInfo(); ok {
		for _, setting := range info.Settings {
			if setting.Key == "vcs.revision" {
				return setting.Value[:7]
			}
		}
	}
	return "dev"
}

func printError(err error) {
	// TODO re-use this in the tests
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
}

func Run(version string) (exitCode int) {
	dbgAst := flag.Bool("dbg-ast", false, "print the AST and exit")
	dbgLex := flag.Bool("dbg-lex", false, "print tokens and exit")
	progFile := flag.String("f", "", "the program file to run")
	rootSelector := flag.String("r", "", "root selector")
	profile := flag.Bool("profile", false, "record a CPU profile")
	outfile := flag.String("o", "", "the file to write JSON to")
	showVersion := flag.Bool("version", false, "print version information")
	flag.Parse()

	if *showVersion {
		fmt.Printf("jqawk %s (revision %s)\n", version, getCommit())
		return 0
	}

	if *profile {
		f, _ := os.Create("jqawk.prof")
		pprof.StartCPUProfile(f)
		defer pprof.StopCPUProfile()
	}

	args := flag.Args()

	var progSrc string
	var filePaths []string
	if len(*progFile) > 0 {
		filePaths = args
		file, err := os.ReadFile(*progFile)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
		progSrc = string(file)
	} else {
		progSrc = flag.Arg(0)
		filePaths = flag.Args()[1:]
	}

	readStdin := false
	if len(filePaths) == 0 && !isatty.IsTerminal(os.Stdin.Fd()) {
		// no files and stdin isn't a tty, read from stdin
		readStdin = true
		filePaths = append(filePaths, "<stdin>")
	}

	// debug args
	if *dbgAst {
		debugAst(progSrc, *rootSelector)
		return 0
	}

	if *dbgLex {
		debugLex(progSrc, *rootSelector)
		return 0
	}

	inputFiles := make([]lang.InputFile, 0)
	for _, filePath := range filePaths {
		if readStdin {
			inputFiles = append(inputFiles, lang.InputFile{
				Name:   "<stdin>",
				Reader: os.Stdin,
			})
		} else {
			fp, err := os.Open(filePath)
			if err != nil {
				fmt.Fprintln(os.Stderr, err)
				return 1
			}
			defer fp.Close()
			inputFiles = append(inputFiles, lang.InputFile{
				Name:   filePath,
				Reader: fp,
			})
		}
	}

	ev, err := lang.EvalProgram(progSrc, inputFiles, *rootSelector, os.Stdout)
	if err != nil {
		printError(err)
		return 1
	}

	if len(*outfile) > 0 {
		if len(filePaths) > 1 {
			fmt.Fprintln(os.Stderr, "error writing JSON: can't write JSON with more than one input file")
			return 1
		}

		j, err := ev.GetRootJson()
		if err != nil {
			fmt.Fprintf(os.Stderr, "error writing JSON: %s\n", err.Error())
			return 1
		}

		if *outfile == "-" {
			fmt.Print(j)
		} else {
			file, err := os.Create(*outfile)
			if err != nil {
				fmt.Fprintf(os.Stderr, "error writing JSON: %s\n", err.Error())
				return 1
			}

			_, err = file.WriteString(j)
			if err != nil {
				fmt.Fprintf(os.Stderr, "error writing JSON: %s\n", err.Error())
				return 1
			}
		}
	}

	return 0
}
