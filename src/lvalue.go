package lang

type LValue interface {
	Get() Value
	Set(Value)
}

type varLValue struct {
	scope *scope
	name  string
}

func (lv varLValue) Get() Value {
	return lv.scope.bindings[lv.name]
}
func (lv varLValue) Set(v Value) {
	lv.scope.bindings[lv.name] = v
}

type objectLValue struct {
	obj *Object
	key string
}

func (lv objectLValue) Get() Value {
	v, _ := lv.obj.Get(lv.key)
	return v
}
func (lv objectLValue) Set(v Value) {
	lv.obj.Set(lv.key, v)
}

type arrayLValue struct {
	arr   *Array
	index int
}

func (lv arrayLValue) Get() Value {
	return lv.arr.Items[lv.index]
}
func (lv arrayLValue) Set(v Value) {
	lv.arr.Items[lv.index] = v
}

type rootLValue struct {
	e    *Evaluator
	slot LValue
}

func (lv rootLValue) Get() Value {
	return *lv.e.ruleRoot
}
func (lv rootLValue) Set(v Value) {
	if lv.slot != nil {
		lv.slot.Set(v)
	}
	lv.e.ruleRoot = &v
}
