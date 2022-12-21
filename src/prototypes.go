package lang

var arrayPrototype *Value = nil
var objPrototype *Value = nil
var strPrototype *Value = nil

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
		}
		strPrototype = &Value{
			Tag: ValueObj,
			Obj: &proto,
		}
	}
	return strPrototype
}