package lang

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
)

type jsonParser struct {
	dec    *json.Decoder
	reader io.Reader
}

func newJsonParser(reader io.Reader) jsonParser {
	dec := json.NewDecoder(reader)
	return jsonParser{dec, reader}
}

func (p *jsonParser) skipToNextValue() error {
	// recover anything the decoder read ahead
	rdr := bufio.NewReader(io.MultiReader(p.dec.Buffered(), p.reader))

	for {
		b, err := rdr.ReadByte()
		switch {
		case err != nil && err != io.EOF:
			return err
		case err == io.EOF:
			p.reader = rdr
			p.dec = json.NewDecoder(rdr)
			return nil
		case b == '\n':
			p.reader = rdr
			p.dec = json.NewDecoder(rdr)
			return nil
		case b == '\r':
			next, err := rdr.ReadByte()
			if err != nil || next != '\n' {
				return fmt.Errorf("expected newline after JSON value")
			}
			p.reader = rdr
			p.dec = json.NewDecoder(rdr)
			return nil
		case b == ' ' || b == '\t':
			// eat whitespace
		default:
			return fmt.Errorf("expected end of JSON value but got '%c'", b)
		}
	}
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

		obj.Obj.Set(key, val)
	}

	tok, err := p.dec.Token()
	if err == io.EOF {
		return Value{}, fmt.Errorf("expected end of JSON input")
	}

	if err != nil {
		return Value{}, err
	}

	if tok != json.Delim('}') {
		return Value{}, fmt.Errorf("expected '}'")
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
		array.Array.Add(val)
	}

	tok, err := p.dec.Token()
	if err == io.EOF {
		return Value{}, fmt.Errorf("unexpected end of JSON input")
	}

	if err != nil {
		return Value{}, err
	}

	if tok != json.Delim(']') {
		return Value{}, fmt.Errorf("expected ']'")
	}

	return array, nil
}
