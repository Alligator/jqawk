package lang

import (
	"encoding/json"
	"fmt"
	"slices"
	"strconv"
	"strings"
	"unicode"
)

func checkArg(args []*Value, index int, tags ...ValueTag) (*Value, error) {
	if len(args)-1 < index {
		return nil, fmt.Errorf("missing argument %d", index)
	}

	arg := args[index]
	if slices.Contains(tags, arg.Tag) {
		return arg, nil
	}

	if len(tags) == 1 {
		return nil, fmt.Errorf("expected argument %d to have type %s", index, tags[0])
	}

	var sb strings.Builder
	for i, tag := range tags {
		sb.WriteString(tag.String())
		if i < len(tags)-1 {
			sb.WriteString(", ")
		}
	}

	return nil, fmt.Errorf("expected argument %d to be one of %s", index, sb.String())
}

func checkArgCount(args []*Value, expectedCount int) error {
	if len(args) != expectedCount {
		return fmt.Errorf("expected %d argument(s)", expectedCount)
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

		widthSpec := 0
		precSpec := -1
		padChar := " "
		if unicode.IsDigit(rune(fmtStr[i])) || fmtStr[i] == '-' {
			numEnd := i + 1
			for numEnd < end && unicode.IsDigit(rune(fmtStr[numEnd])) {
				numEnd++
			}
			numStr := fmtStr[i:numEnd]
			num, err := strconv.ParseInt(numStr, 10, 64)
			if err != nil {
				return nil, fmt.Errorf("invalid width specifier")
			}

			widthSpec = int(num)

			// arbitrary limit
			if widthSpec > 65536 || widthSpec < -65536 {
				return nil, fmt.Errorf("width specifier too large")
			}

			i = numEnd
			if numStr[0] == '0' {
				padChar = "0"
			}
		}

		if i < len(fmtStr)-1 && rune(fmtStr[i]) == '.' {
			precStart := i + 1
			numEnd := precStart + 1
			for numEnd < end && unicode.IsDigit(rune(fmtStr[numEnd])) {
				numEnd++
			}

			precStr := fmtStr[precStart:numEnd]
			prec, err := strconv.ParseInt(precStr, 10, 64)
			if err != nil {
				return nil, fmt.Errorf("invalid precision specifier")
			}

			precSpec = int(prec)
			if precSpec > 65536 || precSpec < -65536 {
				return nil, fmt.Errorf("precision specifier too large")
			}

			i = numEnd
		}

		if i > end-1 {
			return nil, fmt.Errorf("expected something after width specifier")
		}

		pad := func(argStr string) string {
			if widthSpec > 0 && len(argStr) < widthSpec {
				return strings.Repeat(padChar, widthSpec-len(argStr)) + argStr
			}

			if widthSpec < 0 && len(argStr) < -widthSpec {
				return argStr + strings.Repeat(padChar, -widthSpec-len(argStr))
			}

			return argStr
		}

		switch fmtStr[i] {
		case '%':
			sb.WriteByte('%')
		case 'c':
			arg, err := checkArg(args, argIndex, ValueNum)
			if err != nil {
				return nil, err
			}
			argIndex++
			argStr := string(rune(int64(*arg.Num)))
			argStr = pad(argStr)
			sb.WriteString(argStr)
		case 'd', 'i', 'o', 'x':
			arg, err := checkArg(args, argIndex, ValueNum)
			if err != nil {
				return nil, err
			}
			argIndex++

			base := 10
			switch fmtStr[i] {
			case 'o':
				base = 8
			case 'x':
				base = 16
			}

			argStr := strconv.FormatInt(int64(*arg.Num), base)
			argStr = pad(argStr)
			sb.WriteString(argStr)
		case 'f':
			arg, err := checkArg(args, argIndex, ValueNum)
			if err != nil {
				return nil, err
			}
			argIndex++
			argStr := strconv.FormatFloat(*arg.Num, 'f', precSpec, 64)
			argStr = pad(argStr)
			sb.WriteString(argStr)
		case 's':
			arg, err := checkArg(args, argIndex, ValueStr)
			if err != nil {
				return nil, err
			}
			argIndex++
			argStr := arg.String()

			if precSpec != -1 && len(argStr) > precSpec {
				argStr = argStr[:precSpec]
			}

			argStr = pad(argStr)
			sb.WriteString(argStr)
		case 'v':
			if len(args)-1 < argIndex {
				return nil, fmt.Errorf("missing argument %d", argIndex)
			}
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

	bytes, err := json.MarshalIndent(args[0], "", "  ")
	if err != nil {
		if marshalerErr, ok := err.(*json.MarshalerError); ok {
			return nil, fmt.Errorf("error creating JSON: %s", marshalerErr.Unwrap().Error())
		}
		return nil, fmt.Errorf("error creating JSON: %s", err.Error())
	}

	v := NewValue(string(bytes))
	return &v, nil
}

func nativeParseJson(e *Evaluator, args []*Value, this *Value) (*Value, error) {
	s, err := checkArg(args, 0, ValueStr)
	if err != nil {
		return nil, err
	}

	jp := newJsonParser(strings.NewReader(*s.Str))
	val, err := jp.next()
	if err != nil {
		return nil, err
	}
	return &val, err
}

func nativeNum(e *Evaluator, args []*Value, this *Value) (*Value, error) {
	if err := checkArgCount(args, 1); err != nil {
		return nil, err
	}

	switch args[0].Tag {
	case ValueNum:
		v := NewValue(*args[0].Num)
		return &v, nil
	case ValueBool:
		v := NewValue(0)
		if *args[0].Bool {
			v = NewValue(1)
		}
		return &v, nil
	case ValueStr:
		n, err := strconv.ParseFloat(*args[0].Str, 64)
		if err != nil {
			v := NewValue(nil)
			return &v, nil
		}
		v := NewValue(n)
		return &v, nil
	default:
		v := NewValue(nil)
		return &v, nil
	}
}

func addRuntimeFunctions(e *Evaluator) {
	e.setGlobal("printf", Value{
		Tag:      ValueNativeFn,
		NativeFn: nativePrintf,
	})
	e.setGlobal("json", Value{
		Tag:      ValueNativeFn,
		NativeFn: nativeJson,
	})
	e.setGlobal("num", Value{
		Tag:      ValueNativeFn,
		NativeFn: nativeNum,
	})
	e.setGlobal("parseJson", Value{
		Tag:      ValueNativeFn,
		NativeFn: nativeParseJson,
	})
}
