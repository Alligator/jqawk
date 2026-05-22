package cli

import (
	"flag"
	"fmt"
	"io"
	"os"
	"runtime/debug"
	"runtime/pprof"
	"sort"
	"strings"

	lang "github.com/alligator/jqawk/src"
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
		fmt.Fprint(fs.Output(), "usage: jqawk [flags] <program> <file>...\n\n")

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
			placeholder, flagUsage := flag.UnquoteUsage(f)
			name := "-" + f.Name
			if len(f.Name) > 1 {
				name = "--" + f.Name
			}

			if len(placeholder) > 0 {
				name += " <" + placeholder + ">"
			}

			fmt.Fprintf(fs.Output(), "  %-16s %s\n", name, flagUsage)
		}
	}
}

// func Run(version string) (exitCode int) {
func Run(version string, args []string, stdin io.Reader, stdout, stderr io.Writer, isTty bool) (exitCode int) {
	var rValues multiFlag

	fs := flag.NewFlagSet("jqawk", flag.ExitOnError)

	dbgAst := fs.Bool("dbg-ast", false, "print the AST and exit")
	dbgLex := fs.Bool("dbg-lex", false, "print tokens and exit")
	progFile := fs.String("f", "", "the program `file` to run")
	fs.Var(&rValues, "r", "root `selector`. can be specified multiple times")
	profile := fs.Bool("profile", false, "record a CPU profile")
	outfile := fs.String("o", "", "the `file` to write JSON to")
	compact := fs.Bool("c", false, "output compact JSON")
	showVersion := fs.Bool("version", false, "print version information")
	interactive := fs.Bool("i", false, "start interactive REPL")
	expr := fs.String("e", "", "evaluate an `expression` and print the result")

	fs.Usage = usage(fs)
	fs.SetOutput(stderr)

	fs.Parse(args)

	if *showVersion {
		fmt.Fprintf(stdout, "jqawk %s (revision %s)\n", version, getCommit())
		return 0
	}

	if *profile {
		f, _ := os.Create("jqawk.prof")
		pprof.StartCPUProfile(f)
		defer pprof.StopCPUProfile()
	}

	args = fs.Args()

	var progSrc string
	var filePaths []string
	if len(*progFile) > 0 {
		filePaths = args
		file, err := os.ReadFile(*progFile)
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
		progSrc = string(file)
	} else if *interactive || len(*expr) > 0 {
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

	// debug args
	if *dbgAst {
		debugAst(progSrc, rValues, stdout)
		return 0
	}

	if *dbgLex {
		debugLex(progSrc, rValues, stdout)
		return 0
	}

	inputFiles := make([]lang.InputFile, 0)
	if len(filePaths) == 0 && !isTty {
		// no files and stdin isn't a tty, read from stdin
		inputFiles = append(inputFiles, lang.NewStreamingInputFile("<stdin>", stdin))
	} else {
		for _, filePath := range filePaths {
			fp, err := os.Open(filePath)
			if err != nil {
				fmt.Fprintln(stderr, err)
				return 1
			}
			defer fp.Close()
			inputFiles = append(inputFiles, lang.NewStreamingInputFile(filePath, fp))
		}
	}

	if *interactive {
		return RunRepl(version, inputFiles, rValues, stdin, stdout, stderr)
	}

	if len(*expr) > 0 {
		ev := lang.NewEmptyEvaluator(stdout)
		err := ev.RunInBeginFileContext(inputFiles, rValues, func() error {
			lex := lang.NewLexer(*expr)
			parser := lang.NewParser(&lex)
			expr, err := parser.ParseExpression()
			if err != nil {
				return err
			}

			value, err := ev.EvalExpr(expr)
			if err != nil {
				return err
			}
			fmt.Fprintln(stdout, value.PrettyString(false))
			return nil
		})

		if err != nil {
			lang.PrintError(err, stderr)
			return 1
		}
		return 0
	}

	ev, err := lang.EvalProgram(progSrc, inputFiles, rValues, stdout, false)
	if err != nil {
		lang.PrintError(err, stderr)
		return 1
	}

	if len(*outfile) > 0 {
		if len(filePaths) > 1 {
			fmt.Fprintln(stderr, "error writing JSON: can't write JSON with more than one input file")
			return 1
		}

		var j string
		if *compact {
			j, err = ev.GetUglyRootJson()
		} else {
			j, err = ev.GetPrettyRootJson()
		}

		if err != nil {
			fmt.Fprintf(stderr, "error writing JSON: %s\n", err.Error())
			return 1
		}

		if *outfile == "-" {
			fmt.Fprint(stdout, j)
		} else {
			file, err := os.Create(*outfile)
			if err != nil {
				fmt.Fprintf(stderr, "error writing JSON: %s\n", err.Error())
				return 1
			}

			_, err = file.WriteString(j)
			if err != nil {
				fmt.Fprintf(stderr, "error writing JSON: %s\n", err.Error())
				return 1
			}
		}
	}

	return 0
}
