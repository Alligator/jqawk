package lang

type LValue interface {
	Get() Value
	Set(Value) error
}

type varLValue struct {
	scope *scope
	name  string
}

func (lv varLValue) Get() Value {
	return lv.scope.bindings[lv.name]
}
func (lv varLValue) Set(v Value) error {
	lv.scope.bindings[lv.name] = v
	return nil
}

type rootLValue struct {
	e *Evaluator
}

func (lv rootLValue) Get() Value {
	return *lv.e.ruleRoot
}
func (lv rootLValue) Set(v Value) error {
	lv.e.ruleRoot = &v
	return nil
}

type objectLValue struct {
	obj *Object
	key string
}

func (lv objectLValue) Get() Value {
	v, _ := lv.obj.Get(lv.key)
	return v
}
func (lv objectLValue) Set(v Value) error {
	lv.obj.Set(lv.key, v)
	return nil
}

type arrayLValue struct {
	arr   *Array
	index int
}

func (lv arrayLValue) Get() Value {
	return lv.arr.Items[lv.index]
}
func (lv arrayLValue) Set(v Value) error {
	lv.arr.Items[lv.index] = v
	return nil
}
