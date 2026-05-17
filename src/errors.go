package lang

import (
	"fmt"
	"io"
	"strings"
)

type SyntaxError struct {
	Message string
	Line    int
	Col     int
	SrcLine string
}

func (err SyntaxError) Error() string {
	return err.Message
}

type RuntimeError struct {
	Message string
	Line    int
	Col     int
	SrcLine string
}

func (err RuntimeError) Error() string {
	return err.Message
}

type JsonError struct {
	Message  string
	FileName string
}

func (err JsonError) Error() string {
	return err.Message
}

type ErrorGroup struct {
	Errors []error
}

func (err ErrorGroup) Error() string {
	var sb strings.Builder
	for i, err2 := range err.Errors {
		if i > 0 {
			sb.WriteRune('\n')
		}
		sb.WriteString(err2.Error())
	}
	return sb.String()
}

func prefix(line string, col int) string {
	var sb strings.Builder
	for i := range col {
		if line[i] == '\t' {
			sb.WriteRune('\t')
		} else {
			sb.WriteRune(' ')
		}
	}
	return sb.String()
}

func PrintError(err error, dest io.Writer) {
	switch tErr := err.(type) {
	case SyntaxError:
		fmt.Fprintf(dest, "  %s\n", tErr.SrcLine)
		fmt.Fprintf(dest, "  %s%s\n", prefix(tErr.SrcLine, tErr.Col), "^")
		fmt.Fprintf(dest, "syntax error on line %d: %s\n", tErr.Line, tErr.Message)
	case RuntimeError:
		fmt.Fprintf(dest, "  %s\n", tErr.SrcLine)
		fmt.Fprintf(dest, "  %s%s\n", prefix(tErr.SrcLine, tErr.Col), "^")
		fmt.Fprintf(dest, "runtime error on line %d: %s\n", tErr.Line, tErr.Message)
	case JsonError:
		fmt.Fprintf(dest, "could not parse %s: %s\n", tErr.FileName, tErr.Message)
	case ErrorGroup:
		for _, err := range tErr.Errors {
			PrintError(err, dest)
		}
	default:
		fmt.Fprintln(dest, err)
	}
}
