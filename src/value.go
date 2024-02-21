package lang

import (
	"fmt"
	"strconv"
	"strings"
)

// everything in jqawk is wrapped in a Cell
// this adds a layer of indirection so assignment works with any expression
// e.g. a[2] returns a pointer to the cell at a[2], not the value
// then a[2] = 4 just redirects that cell to the new value
type Cell struct {
	Value Value
}

// Create a new cell, taking overship of the value
func NewCell(v Value) *Cell {
	return &Cell{v}
}

type ValueTag uint8

//go:generate stringer -type=ValueTag -linecomment
const (
	ValueStr      ValueTag = iota // string
	ValueBool                     // bool
	ValueNum                      // number
	ValueArray                    // array
	ValueObj                      // object
	ValueNil                      // nil
	ValueNativeFn                 // nativefunction
	ValueFn                       // function
	ValueRegex                    // regex
	ValueUnknown                  // unknown
)

type Value struct {
	Tag       ValueTag
	Str       *string // used by ValueStr and ValueRegex
	Num       *float64
	Bool      *bool
	Array     []*Cell
	Obj       *map[string]*Cell
	NativeFn  func(*Evaluator, []*Value, *Value) (*Value, error)
	Fn        *ExprFunction
	Proto     *Value
	Binding   *Value
	ParentObj *Value
}

func NewValue(srcVal interface{}) Value {
	switch val := srcVal.(type) {
	case []*Cell:
		return Value{
			Tag:   ValueArray,
			Array: val,
			Proto: getArrayPrototype(),
		}
	case []interface{}:
		arr := make([]*Cell, 0, len(val))
		for _, item := range val {
			arr = append(arr, NewCell(NewValue(item)))
		}
		return Value{
			Tag:   ValueArray,
			Array: arr,
			Proto: getArrayPrototype(),
		}
	case []string:
		arr := make([]*Cell, 0, len(val))
		for _, item := range val {
			arr = append(arr, NewCell(NewValue(item)))
		}
		return Value{
			Tag:   ValueArray,
			Array: arr,
			Proto: getArrayPrototype(),
		}
	case map[string]interface{}:
		obj := make(map[string]*Cell)
		for k, v := range val {
			obj[k] = NewCell(NewValue(v))
		}
		return Value{
			Tag:   ValueObj,
			Obj:   &obj,
			Proto: getObjPrototype(),
		}
	case bool:
		return Value{
			Tag:  ValueBool,
			Bool: &val,
		}
	case float64:
		return Value{
			Tag: ValueNum,
			Num: &val,
		}
	case int:
		f := float64(val)
		return Value{
			Tag: ValueNum,
			Num: &f,
		}
	case int64:
		f := float64(val)
		return Value{
			Tag: ValueNum,
			Num: &f,
		}
	case string:
		return NewString(val)
	case nil:
		return Value{
			Tag: ValueNil,
		}
	default:
		panic(fmt.Errorf("unhandled value constructor %T", val))
	}
}

func NewArray() Value {
	arr := make([]*Cell, 0)
	return Value{
		Tag:   ValueArray,
		Array: arr,
		Proto: getArrayPrototype(),
	}
}

func NewObject() Value {
	obj := make(map[string]*Cell)
	return Value{
		Tag:   ValueObj,
		Obj:   &obj,
		Proto: getObjPrototype(),
	}
}

func NewString(str string) Value {
	return Value{
		Tag:   ValueStr,
		Str:   &str,
		Proto: getStrPrototype(),
	}
}

// convert a value to a string suitable for string concatentation, object
// indexing, etc
func (v *Value) String() string {
	switch v.Tag {
	case ValueStr:
		return *v.Str
	case ValueNum:
		return strconv.FormatFloat(*v.Num, 'f', -1, 64)
	default:
		return ""
	}
}

// convert a value to prettified string
func (v *Value) PrettyString(quote bool) string {
	rootValues := make([]*Value, 0)
	return v.prettyStringInteral(rootValues, quote, false)
}

func isSame(a *Value, b *Value) bool {
	if a.Tag != b.Tag {
		return false
	}
	if a.Tag == ValueObj || a.Tag == ValueArray {
		return a.Obj == b.Obj
	}
	return false
}

func (v *Value) prettyStringInteral(rootValues []*Value, quote bool, checkCircularReference bool) string {
	if checkCircularReference {
		for _, rootValue := range rootValues {
			if isSame(rootValue, v) {
				return "<circular reference>"
			}
		}
	}

	switch v.Tag {
	case ValueStr:
		if quote {
			return "\"" + *v.Str + "\""
		}
		return *v.Str
	case ValueNum:
		return strconv.FormatFloat(*v.Num, 'f', -1, 64)
	case ValueBool:
		if *v.Bool {
			return "true"
		}
		return "false"
	case ValueNil:
		return "null"
	case ValueArray:
		var sb strings.Builder
		sb.WriteByte('[')
		for index, cell := range v.Array {
			if index > 0 {
				sb.WriteString(", ")
			}
			sb.WriteString(cell.Value.prettyStringInteral(append(rootValues, v), true, true))
		}
		sb.WriteByte(']')
		return sb.String()
	case ValueObj:
		var sb strings.Builder
		sb.WriteByte('{')
		index := 0
		for key, value := range *v.Obj {
			if index > 0 {
				sb.WriteString(", ")
			}

			sb.WriteString("\"" + key + "\"")
			sb.WriteString(": ")
			sb.WriteString(value.Value.prettyStringInteral(append(rootValues, v), true, true))
			index++
		}
		sb.WriteByte('}')
		return sb.String()
	default:
		return fmt.Sprintf("<%s>", v.Tag.String())
	}
}

func (v *Value) GetMember(member Value) (*Cell, error) {
	switch v.Tag {
	case ValueArray:
		if member.Tag != ValueNum && v.Proto != nil {
			return v.Proto.GetMember(member)
		}
		index := int(*member.Num)
		arr := v.Array

		if index < 0 {
			index = len(arr) + index
			if index < 0 {
				// walked backwards off the front of the array
				return nil, fmt.Errorf("index out of range")
			}
		}

		if index >= len(arr) {
			// TODO sparse arrays
			// don't fill up to enormous numbers, just bail
			if index > 1024*1024 {
				return nil, fmt.Errorf("index too large to auto-fill array")
			}

			// fill the array with empty cells up to the index
			var lastCell *Cell
			for i := len(arr); i <= index; i++ {
				lastCell = NewCell(NewValue(nil))
				arr = append(arr, lastCell)
			}
			v.Array = arr
			return lastCell, nil
		}
		return arr[index], nil
	case ValueObj:
		if member.Tag != ValueNum && member.Tag != ValueStr {
			return nil, fmt.Errorf("objects can only by indexed with numbers or strings, got %s", member.Tag)
		}
		key := member.String()
		value, present := (*v.Obj)[key]
		if present {
			return value, nil
		}
		if v.Proto != nil {
			return v.Proto.GetMember(member)
		}
		return nil, nil
	case ValueStr:
		if member.Tag != ValueNum {
			return v.Proto.GetMember(member)
		}
		index := int(*member.Num)
		if index < 0 || index >= len(*v.Str) {
			return NewCell(NewValue(nil)), nil
		}
		return NewCell(NewString(string((*v.Str)[index]))), nil
	default:
		if v.Proto != nil {
			return v.Proto.GetMember(member)
		}
		return nil, nil
	}
}

func (v *Value) SetMember(member Value, cell *Cell) (*Cell, error) {
	switch v.Tag {
	case ValueArray:
		// TODO
		return nil, fmt.Errorf("setting array members is not yet implemented")
	case ValueObj:
		key := member.String()
		(*v.Obj)[key] = cell
		return cell, nil
	default:
		// TODO?
		return nil, fmt.Errorf("cannot set member on a %s", v.Tag)
	}
}

func (v *Value) isTruthy() bool {
	switch v.Tag {
	case ValueBool:
		return *v.Bool
	case ValueNum:
		return *v.Num != 0.0
	case ValueStr:
		return len(*v.Str) > 0
	case ValueArray, ValueObj, ValueFn, ValueNativeFn: // always truthy
		return true
	}
	return false
}

func (v *Value) asFloat64() float64 {
	switch v.Tag {
	case ValueNum:
		return *v.Num
	case ValueBool:
		if *v.Bool {
			return 1
		}
		return 0
	case ValueStr:
		num, err := strconv.ParseFloat(*v.Str, 64)
		if err != nil {
			return 0
		}
		return num
	}
	return 0
}

func (v *Value) Compare(b *Value) (int, error) {
	// null comparisons
	switch {
	case v.Tag == ValueNil && b.Tag == ValueNil:
		// both null
		return 0, nil
	case v.Tag == ValueNil && b.Tag != ValueNil, v.Tag != ValueNil && b.Tag == ValueNil:
		// one is null
		return 1, nil
	}

	// invalid cases
	if v.Tag == ValueArray || b.Tag == ValueArray || v.Tag == ValueObj || b.Tag == ValueObj {
		return 0, fmt.Errorf("cannot compare %s and %s", v.Tag, b.Tag)
	}

	if v.Tag == ValueStr && b.Tag == ValueStr {
		return strings.Compare(*v.Str, *b.Str), nil
	}

	// coerce to num and compare
	aNum := v.asFloat64()
	bNum := b.asFloat64()
	if aNum > bNum {
		return 1, nil
	} else if aNum < bNum {
		return -1, nil
	} else {
		return 0, nil
	}
}

func (v *Value) Not() *Value {
	var notValue Value
	if v.isTruthy() {
		notValue = NewValue(false)
	} else {
		notValue = NewValue(true)
	}
	return &notValue
}

func (v *Value) ToGoValue() (interface{}, error) {
	rootValues := make([]*Value, 0)
	return v.toGoValueInterval(rootValues, false)
}

func (v *Value) toGoValueInterval(rootValues []*Value, checkCircularReference bool) (interface{}, error) {
	if checkCircularReference {
		for _, rootValue := range rootValues {
			if isSame(rootValue, v) {
				return nil, fmt.Errorf("circular reference")
			}
		}
	}

	switch v.Tag {
	case ValueStr:
		return *v.Str, nil
	case ValueBool:
		return *v.Bool, nil
	case ValueNum:
		return *v.Num, nil
	case ValueArray:
		var array []interface{}
		for _, item := range v.Array {
			val, err := item.Value.toGoValueInterval(append(rootValues, v), true)
			if err != nil {
				return nil, err
			}
			array = append(array, val)
		}
		return array, nil
	case ValueObj:
		obj := make(map[string]interface{})
		for k, objVal := range *v.Obj {
			val, err := objVal.Value.toGoValueInterval(append(rootValues, v), true)
			if err != nil {
				return nil, err
			}
			obj[k] = val
		}
		return obj, nil
	case ValueNil:
		return nil, nil
	default:
		return nil, fmt.Errorf("a %s cannot be converted to a native type", v.Tag)
	}
}
