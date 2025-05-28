package lang

import (
	"encoding/json"
	"fmt"
	"io"
)

type jsonParser struct {
	dec *json.Decoder
}

func newJsonParser(reader io.Reader) jsonParser {
	dec := json.NewDecoder(reader)
	return jsonParser{dec}
}

func (p *jsonParser) next() (Value, error) {
	tok, err := p.dec.Token()
	if err != nil {
		return Value{}, err
	}

	switch v := tok.(type) {
	case json.Delim:
		switch v {
		case '{':
			return p.parseObject()
		case '[':
			return p.parseArray()
		default:
			return Value{}, fmt.Errorf("unexpected delimiter %s", v)
		}
	case string:
		return NewValue(v), nil
	case bool:
		return NewValue(v), nil
	case float64:
		return NewValue(v), nil
	case json.Number:
		f, err := v.Float64()
		if err != nil {
			return Value{}, err
		}
		return NewValue(f), nil
	case nil:
		return NewValue(nil), nil
	default:
		return Value{}, fmt.Errorf("unexpected token %T", v)
	}
}

func (p *jsonParser) parseObject() (Value, error) {
	obj := NewObject()

	for p.dec.More() {
		ktok, err := p.dec.Token()
		if err != nil {
			return Value{}, err
		}

		key, ok := ktok.(string)
		if !ok {
			return Value{}, fmt.Errorf("unexpected string key, got %T", ktok)
		}

		val, err := p.next()
		if err != nil {
			return Value{}, err
		}

		(*obj.Obj)[key] = NewCell(val)
		obj.ObjKeys = append(obj.ObjKeys, key)
	}

	if _, err := p.dec.Token(); err != nil {
		return Value{}, err
	}

	return obj, nil
}

func (p *jsonParser) parseArray() (Value, error) {
	array := NewArray()

	for p.dec.More() {
		val, err := p.next()
		if err != nil {
			return Value{}, err
		}
		array.Array = append(array.Array, NewCell(val))
	}

	if _, err := p.dec.Token(); err != nil {
		return Value{}, err
	}

	return array, nil
}
