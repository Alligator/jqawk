package lang

import (
	"fmt"
	"strconv"
)

type ValueTag uint8

//go:generate stringer -type=ValueTag -linecomment
const (
	ValueStr   ValueTag = iota // string
	ValueBool                  // bool
	ValueNum                   // number
	ValueArray                 // array
	ValueObj                   // object
	ValueNil                   // nil
)

type Value struct {
	Tag   ValueTag
	Str   *string
	Num   *float64
	Bool  *bool
	Array *[]Value
	Obj   *map[string]Value
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

func (v *Value) GetMember(member Value) (Value, error) {
	switch v.Tag {
	case ValueArray:
		if member.Tag != ValueNum {
			return Value{}, fmt.Errorf("arrays can only by indexed with numbers, got %s", member.Tag)
		}
		index := int(*member.Num)
		arr := *v.Array
		if index >= len(arr) {
			return Value{Tag: ValueNil}, nil
		}
		return arr[index], nil
	case ValueObj:
		if member.Tag != ValueNum && member.Tag != ValueStr {
			return Value{}, fmt.Errorf("objects can only by indexed with numbers or strings, got %s", member.Tag)
		}
		key := member.String()
		member, present := (*v.Obj)[key]
		if !present {
			return Value{Tag: ValueNil}, nil
		}
		return member, nil
	default:
		return Value{}, fmt.Errorf("attempted to index a %s", v.Tag)
	}
}

func NewValue(srcVal interface{}) Value {
	switch val := srcVal.(type) {
	case []interface{}:
		arr := make([]Value, 0, len(val))
		for _, item := range val {
			arr = append(arr, NewValue(item))
		}
		return Value{
			Tag:   ValueArray,
			Array: &arr,
		}
	case map[string]interface{}:
		obj := make(map[string]Value)
		for k, v := range val {
			obj[k] = NewValue(v)
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
