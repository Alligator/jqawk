package lang

type LValue interface {
	Get() Value
	Set(Value) error
}

type cellLValue struct {
	cell *Cell
}

func (lv cellLValue) Get() Value {
	return lv.cell.Value
}

func (lv cellLValue) Set(v Value) error {
	lv.cell.Value = v
	return nil
}
