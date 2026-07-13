package lang

import (
	"cmp"
	"fmt"
	"math"
	"slices"
	"strconv"
	"strings"
)

var arrayPrototype *Value = nil
var objPrototype *Value = nil
var strPrototype *Value = nil
var numPrototype *Value = nil

// prototypes are created as prototypeless objects
// this avoids having object methods on everything with a prototype
func makeProto() Value {
	return Value{
		Tag: ValueObj,
		Obj: &Object{make(map[string]*Value), make([]string, 0)},
	}
}

func getArrayPrototype() *Value {
	if arrayPrototype == nil {
		proto := makeProto()

		proto.Obj.Set("length", Value{
			Tag: ValueNativeFn,
			NativeFn: func(e *Evaluator, v []*Value, this *Value) (*Value, error) {
				if this == nil {
					v := NewValue(0)
					return &v, nil
				}

				if this.Tag != ValueArray {
					v := NewValue(0)
					return &v, nil
				}

				length := len(this.Array.Items)
				lengthVal := NewValue(length)
				return &lengthVal, nil
			},
		})

		proto.Obj.Set("push", Value{
			Tag: ValueNativeFn,
			NativeFn: func(e *Evaluator, v []*Value, this *Value) (*Value, error) {
				if this == nil {
					return nil, nil
				}
				if err := checkArgCount(v, 1); err != nil {
					return nil, err
				}

				this.Array.Add(*v[0])
				return this, nil
			},
		})

		proto.Obj.Set("pop", Value{
			Tag: ValueNativeFn,
			NativeFn: func(e *Evaluator, v []*Value, this *Value) (*Value, error) {
				if this == nil {
					return nil, nil
				}
				if err := checkArgCount(v, 0); err != nil {
					return nil, err
				}

				if len(this.Array.Items) == 0 {
					retVal := NewValue(nil)
					return &retVal, nil
				}

				retVal := this.Array.Items[len(this.Array.Items)-1]
				this.Array.Items = this.Array.Items[:len(this.Array.Items)-1]
				return retVal, nil
			},
		})

		proto.Obj.Set("popfirst", Value{
			Tag: ValueNativeFn,
			NativeFn: func(e *Evaluator, v []*Value, this *Value) (*Value, error) {
				if this == nil {
					return nil, nil
				}
				if err := checkArgCount(v, 0); err != nil {
					return nil, err
				}

				if len(this.Array.Items) == 0 {
					retVal := NewValue(nil)
					return &retVal, nil
				}

				retVal := this.Array.Items[0]
				this.Array.Items = this.Array.Items[1:]
				return retVal, nil
			},
		})

		proto.Obj.Set("contains", Value{
			Tag: ValueNativeFn,
			NativeFn: func(e *Evaluator, v []*Value, this *Value) (*Value, error) {
				if this == nil {
					return nil, nil
				}
				if err := checkArgCount(v, 1); err != nil {
					return nil, err
				}

				for _, item := range this.Array.Items {
					comp, err := v[0].Compare(item)
					if err != nil {
						return nil, err
					}
					if comp == 0 {
						retVal := NewValue(true)
						return &retVal, nil
					}
				}

				retVal := NewValue(false)
				return &retVal, nil
			},
		})

		proto.Obj.Set("sort", Value{
			Tag: ValueNativeFn,
			NativeFn: func(e *Evaluator, v []*Value, this *Value) (*Value, error) {
				if this == nil {
					return nil, nil
				}

				clone := this.Array.Clone()

				// is there a sort func?
				if len(v) == 1 {
					if v[0].Tag != ValueFn && v[0].Tag != ValueNativeFn {
						return nil, fmt.Errorf("expected a function")
					}
					sortFunc := v[0]

					var err error
					slices.SortStableFunc(clone.Array.Items, func(a *Value, b *Value) int {
						if err != nil {
							return 0
						}

						var result Value
						result, err = e.callFunction(*sortFunc, []*Value{a, b})
						if err != nil {
							return 0
						}

						if result.Tag == ValueNum {
							return int(*result.Num)
						}

						return 0
					})

					if err != nil {
						return nil, err
					}
				}

				// is this array only numbers?
				onlyNumbers := true
				for _, item := range this.Array.Items {
					if item.Tag != ValueNum {
						onlyNumbers = false
						break
					}
				}

				slices.SortStableFunc(clone.Array.Items, func(a *Value, b *Value) int {
					if onlyNumbers {
						return cmp.Compare(*a.Num, *b.Num)
					}
					return cmp.Compare(a.String(), b.String())
				})

				retVal := clone
				return &retVal, nil
			},
		})

		proto.Obj.Set("sortKey", Value{
			Tag: ValueNativeFn,
			NativeFn: func(e *Evaluator, v []*Value, this *Value) (*Value, error) {
				if this == nil {
					return nil, nil
				}

				clone := this.Array.Clone()

				val, err := checkArg(v, 0, ValueStr, ValueFn, ValueNativeFn)
				if err != nil {
					return nil, err
				}

				switch val.Tag {
				case ValueStr:
					var err error
					slices.SortStableFunc(clone.Array.Items, func(a *Value, b *Value) int {
						if err != nil {
							return 0
						}

						aVal, aFound, err2 := a.GetMember(*val)
						if err2 != nil {
							err = err2
							return 0
						}

						if !aFound {
							return -1
						}

						bVal, bFound, err2 := b.GetMember(*val)
						if err2 != nil {
							err = err2
							return 0
						}

						if !bFound {
							return 1
						}

						result, err2 := aVal.Compare(&bVal)
						if err != nil {
							err = err2
							return 0
						}

						return result
					})
				case ValueFn, ValueNativeFn:
					var err error
					slices.SortStableFunc(clone.Array.Items, func(a *Value, b *Value) int {
						if err != nil {
							return 0
						}

						var aVal Value
						aVal, err = e.callFunction(*val, []*Value{a})
						if err != nil {
							return 0
						}

						var bVal Value
						bVal, err = e.callFunction(*val, []*Value{b})
						if err != nil {
							return 0
						}

						result, err2 := aVal.Compare(&bVal)
						if err != nil {
							err = err2
							return 0
						}

						return result
					})
				}

				retVal := clone
				return &retVal, nil
			},
		})

		proto.Obj.Set("reverse", Value{
			Tag: ValueNativeFn,
			NativeFn: func(e *Evaluator, v []*Value, this *Value) (*Value, error) {
				if this == nil {
					return nil, nil
				}

				clone := this.Array.Clone()
				slices.Reverse(clone.Array.Items)

				return &clone, nil
			},
		})

		proto.Obj.Set("map", Value{
			Tag: ValueNativeFn,
			NativeFn: func(e *Evaluator, v []*Value, this *Value) (*Value, error) {
				if this == nil {
					return nil, nil
				}

				fn, err := checkArg(v, 0, ValueFn, ValueNativeFn)
				if err != nil {
					return nil, err
				}

				output := NewArray()
				for _, val := range this.Array.Items {
					result, err := e.callFunction(*fn, []*Value{val})
					if err != nil {
						return nil, err
					}
					output.Array.Add(result)
				}

				return &output, nil
			},
		})

		proto.Obj.Set("filter", Value{
			Tag: ValueNativeFn,
			NativeFn: func(e *Evaluator, v []*Value, this *Value) (*Value, error) {
				if this == nil {
					return nil, nil
				}

				fn, err := checkArg(v, 0, ValueFn, ValueNativeFn)
				if err != nil {
					return nil, err
				}

				output := NewArray()
				for _, val := range this.Array.Items {
					result, err := e.callFunction(*fn, []*Value{val})
					if err != nil {
						return nil, err
					}
					if result.isTruthy() {
						output.Array.Add(*val)
					}
				}

				return &output, nil
			},
		})

		arrayPrototype = &proto
	}
	return arrayPrototype
}

func getObjPrototype() *Value {
	if objPrototype == nil {
		proto := makeProto()

		proto.Obj.Set("length", Value{
			Tag: ValueNativeFn,
			NativeFn: func(e *Evaluator, v []*Value, this *Value) (*Value, error) {
				if this == nil {
					v := NewValue(0)
					return &v, nil
				}

				if this.Tag != ValueObj {
					v := NewValue(0)
					return &v, nil
				}

				length := len(this.Obj.Items)
				lengthVal := NewValue(length)
				return &lengthVal, nil
			},
		})

		proto.Obj.Set("pluck", Value{
			Tag: ValueNativeFn,
			NativeFn: func(e *Evaluator, v []*Value, this *Value) (*Value, error) {
				newObj := NewObject()
				for _, value := range v {
					val, present, err := this.GetMember(*value)
					if err != nil {
						return nil, err
					}

					if !present {
						newObj.Obj.Set(value.String(), NewValue(nil))
					} else {
						newObj.Obj.Set(value.String(), val)
					}

					if err != nil {
						return nil, err
					}
				}
				return &newObj, nil
			},
		})

		proto.Obj.Set("pairs", Value{
			Tag: ValueNativeFn,
			NativeFn: func(e *Evaluator, v []*Value, this *Value) (*Value, error) {
				newArray := NewArray()
				for _, key := range this.Obj.Keys {
					v, _ := this.Obj.Get(key)
					pair := []Value{NewValue(key), *v}
					newArray.Array.Add(NewValue(pair))
				}
				return &newArray, nil
			},
		})

		proto.Obj.Set("keys", Value{
			Tag: ValueNativeFn,
			NativeFn: func(e *Evaluator, v []*Value, this *Value) (*Value, error) {
				newArray := NewArray()
				for _, key := range this.Obj.Keys {
					newArray.Array.Add(NewValue(key))
				}
				return &newArray, nil
			},
		})

		proto.Obj.Set("values", Value{
			Tag: ValueNativeFn,
			NativeFn: func(e *Evaluator, v []*Value, this *Value) (*Value, error) {
				newArray := NewArray()
				for _, key := range this.Obj.Keys {
					v, _ := this.Obj.Get(key)
					newArray.Array.Add(*v)
				}
				return &newArray, nil
			},
		})

		objPrototype = &proto
	}
	return objPrototype
}

func getStrPrototype() *Value {
	if strPrototype == nil {
		proto := makeProto()

		proto.Obj.Set("length", Value{
			Tag: ValueNativeFn,
			NativeFn: func(e *Evaluator, v []*Value, this *Value) (*Value, error) {
				if this == nil {
					v := NewValue(0)
					return &v, nil
				}

				if this.Tag != ValueStr {
					v := NewValue(0)
					return &v, nil
				}

				length := len(*this.Str)
				lengthVal := NewValue(length)
				return &lengthVal, nil
			},
		})

		proto.Obj.Set("split", Value{
			Tag: ValueNativeFn,
			NativeFn: func(e *Evaluator, v []*Value, this *Value) (*Value, error) {
				if this == nil || this.Tag != ValueStr {
					v := NewArray()
					return &v, nil
				}

				arg, err := checkArg(v, 0, ValueStr, ValueRegex)
				if err != nil {
					return nil, err
				}

				if arg.Tag == ValueStr {
					splits := NewValue(strings.Split(*this.Str, *arg.Str))
					return &splits, nil
				}

				if arg.Tag == ValueRegex {
					re := arg.Regexp
					splits := NewValue(re.Split(*this.Str, -1))
					return &splits, nil
				}

				return nil, nil
			},
		})

		proto.Obj.Set("lower", Value{
			Tag: ValueNativeFn,
			NativeFn: func(e *Evaluator, v []*Value, this *Value) (*Value, error) {
				if this == nil || this.Tag != ValueStr {
					v := NewValue(0)
					return &v, nil
				}

				lower := NewValue(strings.ToLower(*this.Str))
				return &lower, nil
			},
		})

		proto.Obj.Set("upper", Value{
			Tag: ValueNativeFn,
			NativeFn: func(e *Evaluator, v []*Value, this *Value) (*Value, error) {
				if this == nil || this.Tag != ValueStr {
					v := NewValue(0)
					return &v, nil
				}

				upper := NewValue(strings.ToUpper(*this.Str))
				return &upper, nil
			},
		})

		proto.Obj.Set("trim", Value{
			Tag: ValueNativeFn,
			NativeFn: func(e *Evaluator, v []*Value, this *Value) (*Value, error) {
				if this == nil || this.Tag != ValueStr {
					v := NewValue(0)
					return &v, nil
				}

				trimmed := NewValue(strings.TrimSpace(*this.Str))
				return &trimmed, nil
			},
		})

		strPrototype = &proto
	}
	return strPrototype
}

func getNumPrototype() *Value {
	if numPrototype == nil {
		proto := makeProto()

		proto.Obj.Set("floor", Value{
			Tag: ValueNativeFn,
			NativeFn: func(e *Evaluator, v []*Value, this *Value) (*Value, error) {
				if this == nil || this.Tag != ValueNum {
					v := NewValue(nil)
					return &v, nil
				}

				result := NewValue(math.Floor(*this.Num))
				return &result, nil
			},
		})

		proto.Obj.Set("ceil", Value{
			Tag: ValueNativeFn,
			NativeFn: func(e *Evaluator, v []*Value, this *Value) (*Value, error) {
				if this == nil || this.Tag != ValueNum {
					v := NewValue(nil)
					return &v, nil
				}

				result := NewValue(math.Ceil(*this.Num))
				return &result, nil
			},
		})

		proto.Obj.Set("round", Value{
			Tag: ValueNativeFn,
			NativeFn: func(e *Evaluator, v []*Value, this *Value) (*Value, error) {
				if this == nil || this.Tag != ValueNum {
					v := NewValue(nil)
					return &v, nil
				}

				result := NewValue(math.Round(*this.Num))
				return &result, nil
			},
		})

		proto.Obj.Set("mod", Value{
			Tag: ValueNativeFn,
			NativeFn: func(e *Evaluator, v []*Value, this *Value) (*Value, error) {
				arg, err := checkArg(v, 0, ValueNum)
				if err != nil {
					return nil, err
				}

				if this == nil || this.Tag != ValueNum {
					v := NewValue(nil)
					return &v, nil
				}

				a := *this.Num
				b := *arg.Num
				result := NewValue(math.Mod((math.Mod(a, b) + b), b))
				return &result, nil
			},
		})

		proto.Obj.Set("abs", Value{
			Tag: ValueNativeFn,
			NativeFn: func(e *Evaluator, v []*Value, this *Value) (*Value, error) {
				if this == nil || this.Tag != ValueNum {
					v := NewValue(nil)
					return &v, nil
				}

				result := NewValue(math.Abs(*this.Num))
				return &result, nil
			},
		})

		proto.Obj.Set("format", Value{
			Tag: ValueNativeFn,
			NativeFn: func(e *Evaluator, v []*Value, this *Value) (*Value, error) {
				thousandsSep := ","
				decimalSep := "."
				if len(v) == 2 {
					thousandsSep = v[0].String()
					decimalSep = v[1].String()
				} else if len(v) == 1 {
					return nil, fmt.Errorf("expected a thousands and decimal separator")
				}

				numStr := strconv.FormatFloat(*this.Num, 'f', -1, 64)
				parts := strings.Split(numStr, ".")

				var sb strings.Builder
				if len(parts[0]) <= 3 {
					sb.WriteString(parts[0])
				} else {
					charsUntilSep := len(parts[0]) % 3
					if charsUntilSep == 0 {
						charsUntilSep = 3
					}

					for _, r := range parts[0] {
						if charsUntilSep == 0 {
							sb.WriteString(thousandsSep)
							charsUntilSep = 2
						} else {
							charsUntilSep--
						}

						sb.WriteRune(r)
					}
				}

				if len(parts) == 2 {
					sb.WriteString(decimalSep)
					sb.WriteString(parts[1])
				}

				val := NewValue(sb.String())
				return &val, nil
			},
		})

		numPrototype = &proto
	}
	return numPrototype
}
