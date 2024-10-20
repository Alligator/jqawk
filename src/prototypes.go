package lang

import (
	"cmp"
	"math"
	"slices"
	"strings"
)

var arrayPrototype *Value = nil
var objPrototype *Value = nil
var strPrototype *Value = nil
var numPrototype *Value = nil

func getArrayPrototype() *Value {
	if arrayPrototype == nil {
		proto := map[string]*Cell{
			"length": NewCell(Value{
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

					length := len(this.Array)
					lengthVal := NewValue(length)
					return &lengthVal, nil
				},
			}),
			"push": NewCell(Value{
				Tag: ValueNativeFn,
				NativeFn: func(e *Evaluator, v []*Value, this *Value) (*Value, error) {
					if this == nil {
						return nil, nil
					}
					if err := checkArgCount(v, 1); err != nil {
						return nil, err
					}

					this.Array = append(this.Array, NewCell(*v[0]))
					return this, nil
				},
			}),
			"pop": NewCell(Value{
				Tag: ValueNativeFn,
				NativeFn: func(e *Evaluator, v []*Value, this *Value) (*Value, error) {
					if this == nil {
						return nil, nil
					}
					if err := checkArgCount(v, 0); err != nil {
						return nil, err
					}

					if len(this.Array) == 0 {
						retVal := NewValue(nil)
						return &retVal, nil
					}

					retVal := this.Array[len(this.Array)-1].Value
					this.Array = this.Array[:len(this.Array)-1]
					return &retVal, nil
				},
			}),
			"popfirst": NewCell(Value{
				Tag: ValueNativeFn,
				NativeFn: func(e *Evaluator, v []*Value, this *Value) (*Value, error) {
					if this == nil {
						return nil, nil
					}
					if err := checkArgCount(v, 0); err != nil {
						return nil, err
					}

					if len(this.Array) == 0 {
						retVal := NewValue(nil)
						return &retVal, nil
					}

					retVal := this.Array[0].Value
					this.Array = this.Array[1:]
					return &retVal, nil
				},
			}),
			"contains": NewCell(Value{
				Tag: ValueNativeFn,
				NativeFn: func(e *Evaluator, v []*Value, this *Value) (*Value, error) {
					if this == nil {
						return nil, nil
					}
					if err := checkArgCount(v, 1); err != nil {
						return nil, err
					}

					for _, item := range this.Array {
						comp, err := v[0].Compare(&item.Value)
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
			}),
			"sort": NewCell(Value{
				Tag: ValueNativeFn,
				NativeFn: func(e *Evaluator, v []*Value, this *Value) (*Value, error) {
					if this == nil {
						return nil, nil
					}

					// is this array only numbers?
					onlyNumbers := true
					for _, item := range this.Array {
						if item.Value.Tag != ValueNum {
							onlyNumbers = false
							break
						}
					}

					// make a clone
					clone := make([]*Cell, len(this.Array))
					for i, item := range this.Array {
						clone[i] = &Cell{}
						copyValue(item, clone[i])
					}

					slices.SortStableFunc(clone, func(a *Cell, b *Cell) int {
						if onlyNumbers {
							return cmp.Compare(*a.Value.Num, *b.Value.Num)
						}
						return cmp.Compare(a.Value.String(), b.Value.String())
					})

					retVal := NewValue(clone)
					return &retVal, nil
				},
			}),
		}
		arrayPrototype = &Value{
			Tag: ValueObj,
			Obj: &proto,
		}
	}
	return arrayPrototype
}

func getObjPrototype() *Value {
	if objPrototype == nil {
		proto := map[string]*Cell{
			"length": NewCell(Value{
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

					length := len(*this.Obj)
					lengthVal := NewValue(length)
					return &lengthVal, nil
				},
			}),
			"pluck": NewCell(Value{
				Tag: ValueNativeFn,
				NativeFn: func(e *Evaluator, v []*Value, this *Value) (*Value, error) {
					newObj := NewObject()
					for _, value := range v {
						val, err := this.GetMember(*value)
						if err != nil {
							return nil, err
						}

						if val == nil {
							_, err = newObj.SetMember(*value, NewCell(NewValue(nil)))
						} else {
							_, err = newObj.SetMember(*value, NewCell(val.Value))
						}

						if err != nil {
							return nil, err
						}
					}
					return &newObj, nil
				},
			}),
		}
		objPrototype = &Value{
			Tag: ValueObj,
			Obj: &proto,
		}
	}
	return objPrototype
}

func getStrPrototype() *Value {
	if strPrototype == nil {
		proto := map[string]*Cell{
			"length": NewCell(Value{
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
			}),
			"split": NewCell(Value{
				Tag: ValueNativeFn,
				NativeFn: func(e *Evaluator, v []*Value, this *Value) (*Value, error) {
					if this == nil || this.Tag != ValueStr {
						v := NewArray()
						return &v, nil
					}

					str, err := checkArg(v, 0, ValueStr)
					if err != nil {
						return nil, err
					}

					splits := NewValue(strings.Split(*this.Str, *str.Str))
					return &splits, nil
				},
			}),
			"lower": NewCell(Value{
				Tag: ValueNativeFn,
				NativeFn: func(e *Evaluator, v []*Value, this *Value) (*Value, error) {
					if this == nil || this.Tag != ValueStr {
						v := NewValue(0)
						return &v, nil
					}

					lower := NewValue(strings.ToLower(*this.Str))
					return &lower, nil
				},
			}),
			"upper": NewCell(Value{
				Tag: ValueNativeFn,
				NativeFn: func(e *Evaluator, v []*Value, this *Value) (*Value, error) {
					if this == nil || this.Tag != ValueStr {
						v := NewValue(0)
						return &v, nil
					}

					upper := NewValue(strings.ToUpper(*this.Str))
					return &upper, nil
				},
			}),
		}
		strPrototype = &Value{
			Tag: ValueObj,
			Obj: &proto,
		}
	}
	return strPrototype
}

func getNumPrototype() *Value {
	if numPrototype == nil {
		proto := map[string]*Cell{
			"floor": NewCell(Value{
				Tag: ValueNativeFn,
				NativeFn: func(e *Evaluator, v []*Value, this *Value) (*Value, error) {
					if this == nil || this.Tag != ValueNum {
						v := NewValue(nil)
						return &v, nil
					}

					result := NewValue(math.Floor(*this.Num))
					return &result, nil
				},
			}),
			"ceil": NewCell(Value{
				Tag: ValueNativeFn,
				NativeFn: func(e *Evaluator, v []*Value, this *Value) (*Value, error) {
					if this == nil || this.Tag != ValueNum {
						v := NewValue(nil)
						return &v, nil
					}

					result := NewValue(math.Ceil(*this.Num))
					return &result, nil
				},
			}),
			"round": NewCell(Value{
				Tag: ValueNativeFn,
				NativeFn: func(e *Evaluator, v []*Value, this *Value) (*Value, error) {
					if this == nil || this.Tag != ValueNum {
						v := NewValue(nil)
						return &v, nil
					}

					result := NewValue(math.Round(*this.Num))
					return &result, nil
				},
			}),
		}
		numPrototype = &Value{
			Tag: ValueObj,
			Obj: &proto,
		}
	}
	return numPrototype
}
