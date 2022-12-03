package lang

import (
	"fmt"
	"strings"
)

func checkArg(args []*Value, index int, tag ValueTag) (*Value, error) {
	if len(args)-1 < index {
		return nil, fmt.Errorf("missing argument %d", index)
	}

	arg := args[index]
	if arg.Tag != tag {
		return nil, fmt.Errorf("expected arguments %d to have type %s", index, tag)
	}

	return arg, nil
}

func checkArgCount(args []*Value, expectedCount int) error {
	if len(args) != expectedCount {
		return fmt.Errorf("expected %d arguments", expectedCount)
	}
	return nil
}

func nativePrintf(e *Evaluator, args []*Value, this *Value) (*Value, error) {
	if len(args) < 1 {
		return nil, fmt.Errorf("printf requires at least one argument")
	}

	fmtVal, err := checkArg(args, 0, ValueStr)
	if err != nil {
		return nil, err
	}

	fmtStr := *fmtVal.Str
	end := len(fmtStr)
	argIndex := 1
	var sb strings.Builder

	for i := 0; i < end; i++ {
		b := fmtStr[i]
		if b != '%' {
			sb.WriteByte(b)
			continue
		}

		if i == end-1 {
			return nil, fmt.Errorf("expected something after %%")
		}
		i++

		switch fmtStr[i] {
		case '%':
			sb.WriteByte('%')
		case 's':
			arg, err := checkArg(args, argIndex, ValueStr)
			if err != nil {
				return nil, err
			}
			argIndex++
			sb.WriteString(arg.String())
		case 'f':
			arg, err := checkArg(args, argIndex, ValueNum)
			if err != nil {
				return nil, err
			}
			argIndex++
			sb.WriteString(arg.String())
		}
	}

	e.print(sb.String())
	return nil, nil
}

func addRuntimeFunctions(e *Evaluator) {
	e.stackTop.locals["printf"] = NewCell(Value{
		Tag:      ValueNativeFn,
		NativeFn: nativePrintf,
	})
}
