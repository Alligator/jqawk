package lang

import (
	"encoding/json"
	"fmt"
	"strconv"
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
		case 'v':
			sb.WriteString(args[argIndex].PrettyString(false))
			argIndex++
		default:
			return nil, fmt.Errorf("unknown format code %c", fmtStr[i])
		}
	}

	e.print(sb.String())
	return nil, nil
}

func nativeJson(e *Evaluator, args []*Value, this *Value) (*Value, error) {
	if err := checkArgCount(args, 1); err != nil {
		return nil, err
	}

	val, err := args[0].ToGoValue()
	if err != nil {
		return nil, err
	}

	bytes, err := json.MarshalIndent(val, "", "  ")
	if err != nil {
		return nil, err
	}

	v := NewValue(string(bytes))
	return &v, nil
}

func nativeInt(e *Evaluator, args []*Value, this *Value) (*Value, error) {
	if err := checkArgCount(args, 1); err != nil {
		return nil, err
	}

	switch args[0].Tag {
	case ValueNum:
		v := NewValue(int(*args[0].Num))
		return &v, nil
	case ValueStr:
		n, err := strconv.ParseInt(*args[0].Str, 10, 64)
		if err != nil {
			// TODO better error message
			return nil, err
		}
		v := NewValue(n)
		return &v, nil
	default:
		v := NewValue(nil)
		return &v, nil
	}
}

func addRuntimeFunctions(e *Evaluator) {
	e.stackTop.locals["printf"] = NewCell(Value{
		Tag:      ValueNativeFn,
		NativeFn: nativePrintf,
	})
	e.stackTop.locals["json"] = NewCell(Value{
		Tag:      ValueNativeFn,
		NativeFn: nativeJson,
	})
	e.stackTop.locals["int"] = NewCell(Value{
		Tag:      ValueNativeFn,
		NativeFn: nativeInt,
	})
}
