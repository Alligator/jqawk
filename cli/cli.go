package cli

import (
	"flag"
	"fmt"
	"os"
	"runtime/debug"
	"runtime/pprof"
	"sort"
	"strings"

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

type multiFlag []string

func (m *multiFlag) String() string {
	return strings.Join(*m, ",")
}

func (m *multiFlag) Set(value string) error {
	*m = append(*m, value)
	return nil
}

func usage(fs *flag.FlagSet) func() {
	return func() {
		fmt.Fprint(fs.Output(), "usage: jqawk [flags] <file>...\n\n")

		var flags []*flag.Flag
		fs.VisitAll(func(f *flag.Flag) {
			flags = append(flags, f)
		})

		// lexicographic but single char flags come first
		sort.Slice(flags, func(a, b int) bool {
			if len(flags[a].Name) == 1 {
				if len(flags[b].Name) == 1 {
					return flags[a].Name < flags[b].Name
				}
				return true
			}

			return flags[a].Name < flags[b].Name
		})

		for _, f := range flags {
			if len(f.Name) == 1 {
				fmt.Fprintf(fs.Output(), "  -%-11s %s\n", f.Name, f.Usage)
			} else {
				fmt.Fprintf(fs.Output(), "  --%-10s %s\n", f.Name, f.Usage)
			}
		}
	}
}

func Run(version string) (exitCode int) {
	var rValues multiFlag

	dbgAst := flag.Bool("dbg-ast", false, "print the AST and exit")
	dbgLex := flag.Bool("dbg-lex", false, "print tokens and exit")
	progFile := flag.String("f", "", "the program file to run")
	flag.Var(&rValues, "r", "root selector. can be specified multiple times")
	profile := flag.Bool("profile", false, "record a CPU profile")
	outfile := flag.String("o", "", "the file to write JSON to")
	showVersion := flag.Bool("version", false, "print version information")
	interactive := flag.Bool("i", false, "start interactive REPL")

	flag.Usage = usage(flag.CommandLine)

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
	} else if *interactive {
		filePaths = args
	} else {
		switch len(args) {
		case 0:
			progSrc = ""
		case 1:
			progSrc = args[0]
		default:
			progSrc = args[0]
			filePaths = args[1:]
		}
	}

	readStdin := false
	if len(filePaths) == 0 && !isatty.IsTerminal(os.Stdin.Fd()) {
		// no files and stdin isn't a tty, read from stdin
		readStdin = true
		filePaths = append(filePaths, "<stdin>")
	}

	// debug args
	if *dbgAst {
		debugAst(progSrc, rValues)
		return 0
	}

	if *dbgLex {
		debugLex(progSrc, rValues)
		return 0
	}

	inputFiles := make([]lang.InputFile, 0)
	for _, filePath := range filePaths {
		if readStdin {
			inputFiles = append(inputFiles, lang.NewStreamingInputFile("<stdin>", os.Stdin))
		} else {
			fp, err := os.Open(filePath)
			if err != nil {
				fmt.Fprintln(os.Stderr, err)
				return 1
			}
			defer fp.Close()
			inputFiles = append(inputFiles, lang.NewStreamingInputFile(filePath, fp))
		}
	}

	if *interactive {
		return RunRepl(version, inputFiles, rValues)
	}

	ev, err := lang.EvalProgram(progSrc, inputFiles, rValues, os.Stdout, false)
	if err != nil {
		lang.PrintError(err)
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
