package lang

import (
	"bytes"
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

type ValueTag uint8

//go:generate stringer -type=ValueTag -linecomment
const (
	ValueStr      ValueTag = iota // string
	ValueBool                     // bool
	ValueNum                      // number
	ValueArray                    // array
	ValueObj                      // object
	ValueNil                      // null
	ValueNativeFn                 // nativefunction
	ValueFn                       // function
	ValueRegex                    // regex
	ValueUnknown                  // unknown
)

type Value struct {
	Tag      ValueTag
	Str      *string
	Num      *float64
	Bool     *bool
	Array    *Array
	Obj      *Object
	NativeFn func(*Evaluator, []*Value, *Value) (*Value, error)
	Fn       FnWithContext
	Regexp   *regexp.Regexp
	Proto    *Value
	Binding  *Value
}

type Array struct {
	Items []*Value
}

func (a *Array) Add(value Value) {
	a.Items = append(a.Items, &value)
}

func (a *Array) Clone() Value {
	clone := NewArray()
	for _, item := range a.Items {
		clone.Array.Add(*item)
	}
	return clone
}

type Object struct {
	Items map[string]*Value
	Keys  []string
}

func (o *Object) Set(key string, cell Value) {
	_, present := o.Items[key]
	o.Items[key] = &cell
	if !present {
		o.Keys = append(o.Keys, key)
	}
}

func (o *Object) Get(key string) (*Value, bool) {
	cell, ok := o.Items[key]
	return cell, ok
}

type FnWithContext struct {
	Expr  *ExprFunction
	Scope *scope
}

func NewValue(srcVal any) Value {
	switch val := srcVal.(type) {
	case []Value:
		arr := NewArray()
		for _, item := range val {
			arr.Array.Add(item)
		}
		return arr
	case []any:
		arr := NewArray()
		for _, item := range val {
			arr.Array.Add(NewValue(item))
		}
		return arr
	case []string:
		arr := NewArray()
		for _, item := range val {
			arr.Array.Add(NewValue(item))
		}
		return arr
	case map[string]any:
		obj := NewObject()
		for k, v := range val {
			obj.Obj.Set(k, NewValue(v))
		}
		return obj
	case bool:
		return Value{
			Tag:  ValueBool,
			Bool: &val,
		}
	case float64:
		return Value{
			Tag:   ValueNum,
			Num:   &val,
			Proto: getNumPrototype(),
		}
	case int:
		f := float64(val)
		return Value{
			Tag:   ValueNum,
			Num:   &f,
			Proto: getNumPrototype(),
		}
	case int64:
		f := float64(val)
		return Value{
			Tag:   ValueNum,
			Num:   &f,
			Proto: getNumPrototype(),
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
	arr := Array{make([]*Value, 0)}
	return Value{
		Tag:   ValueArray,
		Array: &arr,
		Proto: getArrayPrototype(),
	}
}

func NewObject() Value {
	obj := Object{make(map[string]*Value), make([]string, 0)}
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

// check if two value slices have the same underlying array
// borrowed from go's math library
// https://go.dev/src/math/big/nat.go#L374
func alias(x, y []*Value) bool {
	return cap(x) > 0 && cap(y) > 0 && &x[0:cap(x)][cap(x)-1] == &y[0:cap(y)][cap(y)-1]
}

func isSame(a *Value, b *Value) bool {
	if a.Tag != b.Tag {
		return false
	}
	if a.Tag == ValueObj {
		return a.Obj == b.Obj
	}
	if a.Tag == ValueArray && b.Tag == ValueArray {
		// FIXME pointer check?
		return alias(a.Array.Items, b.Array.Items)
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
		for index, value := range v.Array.Items {
			if index > 0 {
				sb.WriteString(", ")
			}
			sb.WriteString(value.prettyStringInteral(append(rootValues, v), true, true))
		}
		sb.WriteByte(']')
		return sb.String()
	case ValueObj:
		var sb strings.Builder
		sb.WriteByte('{')
		index := 0
		for _, key := range v.Obj.Keys {
			value, _ := v.Obj.Get(key)
			if index > 0 {
				sb.WriteString(", ")
			}

			sb.WriteString("\"" + key + "\"")
			sb.WriteString(": ")
			sb.WriteString(value.prettyStringInteral(append(rootValues, v), true, true))
			index++
		}
		sb.WriteByte('}')
		return sb.String()
	default:
		return fmt.Sprintf("<%s>", v.Tag.String())
	}
}

func normalizeIndex(index float64, length int) int {
	i := int(index)
	if i < 0 {
		i = length + i
	}
	return i
}

func getArrayIndex(index float64, length int) (int, bool) {
	i := normalizeIndex(index, length)

	if i < 0 || i >= length {
		return i, false
	}

	return i, true
}

func getSliceIndex(index float64, length int) (int, bool) {
	i := normalizeIndex(index, length)

	if i < 0 {
		return i, false
	}

	if i > length {
		return length, true
	}

	return i, true
}

func (v *Value) GetMember(member Value) (Value, bool, error) {
	// This comment explains the behaviour of GetMember and the reasons why.
	//
	// For arrays
	// - a numerical index looks up that array element, or returns nil if the
	//   index is outside the bounds of the array
	// - anything else attempts a prototype lookup
	//
	// For objects
	// - any key attempts a lookup on the object, if that fails a prototype
	//   lookup is attempted
	//
	// The end result is a missing array item, object property, or method returns
	// null. If a JSON field is sometimes omitted, it's better that $.field
	// evalutes to nullthan causes a runtime error.

	switch v.Tag {
	case ValueArray:
		if member.Tag != ValueNum {
			break
		}

		index, ok := getArrayIndex(*member.Num, len(v.Array.Items))

		if !ok {
			return NewValue(nil), false, nil
		}

		arr := v.Array
		return *arr.Items[index], true, nil
	case ValueObj:
		if member.Tag != ValueNum && member.Tag != ValueStr {
			break
		}

		key := member.String()
		value, present := v.Obj.Get(key)

		if !present {
			break
		}
		return *value, true, nil
	case ValueStr:
		if member.Tag != ValueNum {
			break
		}

		index := int(*member.Num)
		str := *v.Str

		if index < 0 {
			index = len(str) + index
			if index < 0 {
				// walked backwards off the front of the array
				return Value{}, false, fmt.Errorf("index out of range")
			}
		}

		if index < 0 || index >= len(*v.Str) {
			return NewValue(nil), false, nil
		}
		return NewString(string((*v.Str)[index])), true, nil
	}

	if v.Proto != nil {
		result, found, err := v.Proto.GetMember(member)

		if err != nil {
			return Value{}, false, err
		}

		if found {
			return result, found, nil
		}
	}

	return NewValue(nil), false, nil
}

func (v *Value) SetMember(member Value, value Value) error {
	switch v.Tag {
	case ValueArray:
		if member.Tag != ValueNum {
			return fmt.Errorf("array indices must be numbers")
		}

		index, ok := getArrayIndex(*member.Num, len(v.Array.Items))
		if !ok {
			if index < 0 {
				return fmt.Errorf("index out of range")
			}

			// past the end of the array
			if index > 1024*1024 {
				return fmt.Errorf("index too large to auto-fill array")
			}

			lastIndex := 0
			for i := len(v.Array.Items); i <= index; i++ {
				lastIndex = i
				v.Array.Add(NewValue(nil))
			}
			v.Array.Items[lastIndex] = &value

			return nil
		}

		err := v.SetMember(member, value)
		if err != nil {
			return err
		}
		return nil
	case ValueObj:
		key := member.String()
		v.Obj.Set(key, value)
		return nil
	default:
		return fmt.Errorf("cannot set member on a %s", v.Tag)
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
	case v.Tag == ValueNil && b.Tag != ValueNil:
		// a is null
		return -1, nil
	case v.Tag != ValueNil && b.Tag == ValueNil:
		// b is null
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

func (v *Value) MarshalJSON() ([]byte, error) {
	seen := make([]*Value, 0)
	var buf bytes.Buffer
	err := v.marshalAndDetectCircularReferences(&buf, seen)
	if err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func (v *Value) marshalAndDetectCircularReferences(w *bytes.Buffer, seen []*Value) error {
	var b []byte
	var err error

	for _, seenVal := range seen {
		if isSame(seenVal, v) {
			return fmt.Errorf("circular reference")
		}
	}
	seen = append(seen, v)

	switch v.Tag {
	case ValueStr:
		b, err = json.Marshal(v.Str)
	case ValueBool:
		b, err = json.Marshal(v.Bool)
	case ValueNum:
		b, err = json.Marshal(v.Num)
	case ValueNil, ValueUnknown:
		b, err = json.Marshal(nil)
	case ValueArray:
		b, err = json.Marshal(v.Array.Items)
	case ValueObj:
		w.WriteString("{ ")
		for i, key := range v.Obj.Keys {
			if i > 0 {
				w.WriteString(", ")
			}

			keyJson, err := json.Marshal(key)
			if err != nil {
				return err
			}

			w.Write(keyJson)
			w.WriteString(": ")

			value, _ := v.Obj.Get(key)
			err = value.marshalAndDetectCircularReferences(w, seen)
			if err != nil {
				return err
			}
		}
		w.WriteString(" }")
		return nil
	default:
		return fmt.Errorf("unhandled tag %v", v.Tag)
	}

	if err != nil {
		return err
	}

	w.Write(b)
	return nil
}
