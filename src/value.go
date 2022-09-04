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
	ValueStr     ValueTag = iota // string
	ValueBool                    // bool
	ValueNum                     // number
	ValueArray                   // array
	ValueObj                     // object
	ValueNil                     // nil
	ValueUnknown                 // unknown
)

type Value struct {
	Tag   ValueTag
	Str   *string
	Num   *float64
	Bool  *bool
	Array *[]*Cell
	Obj   *map[string]*Cell
}

func NewValue(srcVal interface{}) Value {
	switch val := srcVal.(type) {
	case []interface{}:
		arr := make([]*Cell, 0, len(val))
		for _, item := range val {
			arr = append(arr, NewCell(NewValue(item)))
		}
		return Value{
			Tag:   ValueArray,
			Array: &arr,
		}
	case map[string]interface{}:
		obj := make(map[string]*Cell)
		for k, v := range val {
			obj[k] = NewCell(NewValue(v))
		}
		return Value{
			Tag: ValueObj,
			Obj: &obj,
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
	case string:
		return Value{
			Tag: ValueStr,
			Str: &val,
		}
	case nil:
		return Value{
			Tag: ValueNil,
		}
	default:
		panic(fmt.Errorf("unhandled json value type %T", val))
	}
}

func (v *Value) String() string {
	switch v.Tag {
	case ValueStr:
		return *v.Str
	case ValueNum:
		return strconv.FormatFloat(*v.Num, 'f', -1, 64)
	default:
		return fmt.Sprintf("<%s>", v.Tag.String())
	}
}

func (v *Value) GetMember(member Value) (*Cell, error) {
	switch v.Tag {
	case ValueArray:
		if member.Tag != ValueNum {
			return nil, fmt.Errorf("arrays can only by indexed with numbers, got %s", member.Tag)
		}
		index := int(*member.Num)
		arr := *v.Array
		if index >= len(arr) {
			return NewCell(NewValue(nil)), nil
		}
		return arr[index], nil
	case ValueObj:
		if member.Tag != ValueNum && member.Tag != ValueStr {
			return nil, fmt.Errorf("objects can only by indexed with numbers or strings, got %s", member.Tag)
		}
		key := member.String()
		member, present := (*v.Obj)[key]
		if !present {
			return NewCell(NewValue(nil)), nil
		}
		return member, nil
	default:
		return nil, fmt.Errorf("attempted to index a %s", v.Tag)
	}
}

func (v *Value) isTruthy() bool {
	switch v.Tag {
	case ValueBool:
		return *v.Bool
	case ValueNum:
		return *v.Num > 0.0
	case ValueStr:
		return len(*v.Str) > 0
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
	}
	return 0
}

func (v *Value) Compare(b *Value) (int, error) {
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
