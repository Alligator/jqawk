package lang

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"maps"
	"regexp"
	"strconv"
	"strings"
)

type scope struct {
	parent   *scope
	bindings map[string]Value
}

type stackFrame struct {
	name   string
	depth  int
	scope  *scope
	parent *stackFrame
}

type Evaluator struct {
	prog           Program
	stdout         io.Writer
	root           *Value
	ruleRoot       *Value
	ruleRootSlot   *rootLValue
	stackTop       *stackFrame
	returnVal      *Value
	beginRules     []*Rule
	beginFileRules []*Rule
	patternRules   []*Rule
	endRules       []*Rule
	endFileRules   []*Rule
	fuzzing        bool
	ctx            context.Context
}

var (
	errContinue = errors.New("continue")
	errBreak    = errors.New("break")
	errReturn   = errors.New("return")
	errNext     = errors.New("next")
	errExit     = errors.New("exit")
)

var fuzzingLoopLimit = 10000
var callDepthLimit = 4096

func NewEvaluator(prog Program, stdout io.Writer) Evaluator {
	e := Evaluator{
		prog:   prog,
		stdout: stdout,
	}
	e.readRules()
	e.pushFrame("<root>")
	addRuntimeFunctions(&e)
	e.addProgramFunctions()
	return e
}

func NewEmptyEvaluator(stdout io.Writer) Evaluator {
	e := Evaluator{
		stdout: stdout,
	}
	e.pushFrame("<root>")
	addRuntimeFunctions(&e)
	return e
}

func (e *Evaluator) readRules() {
	e.beginRules = make([]*Rule, 0)
	e.beginFileRules = make([]*Rule, 0)
	e.endRules = make([]*Rule, 0)
	e.endFileRules = make([]*Rule, 0)
	e.patternRules = make([]*Rule, 0)

	for _, rule := range e.prog.Rules {
		r := rule
		switch rule.Kind {
		case BeginRule:
			e.beginRules = append(e.beginRules, &r)
		case BeginFileRule:
			e.beginFileRules = append(e.beginFileRules, &r)
		case EndRule:
			e.endRules = append(e.endRules, &r)
		case EndFileRule:
			e.endFileRules = append(e.endFileRules, &r)
		case PatternRule:
			e.patternRules = append(e.patternRules, &r)
		default:
			panic(fmt.Errorf("unknown rule type %s", rule.Kind))
		}
	}
}

func (e *Evaluator) addProgramFunctions() {
	for _, fn := range e.prog.Functions {
		f := FnWithContext{
			Expr:  &fn,
			Scope: e.stackTop.scope,
		}
		val := Value{
			Tag: ValueFn,
			Fn:  f,
		}
		e.stackTop.scope.bindings[fn.Ident] = val
	}
}

func (e *Evaluator) print(str string) {
	fmt.Fprint(e.stdout, str)
}

func (e *Evaluator) error(token Token, msg string) RuntimeError {
	srcLine, line, col := token.GetLineAndCol()
	return RuntimeError{
		Message: msg,
		Line:    line,
		Col:     col,
		SrcLine: srcLine,
	}
}

func (e *Evaluator) pushFrame(name string) error {
	scope := scope{
		bindings: make(map[string]Value),
	}

	frame := stackFrame{
		name:   name,
		scope:  &scope,
		parent: e.stackTop,
	}

	if e.stackTop != nil {
		scope.parent = e.stackTop.scope
		frame.depth = e.stackTop.depth + 1
	}

	if frame.depth > callDepthLimit {
		return fmt.Errorf("call depth limit exceeded")
	}

	e.stackTop = &frame
	return nil
}

func (e *Evaluator) pushScope() {
	scope := scope{
		bindings: make(map[string]Value),
		parent:   e.stackTop.scope,
	}
	e.stackTop.scope = &scope
}

func (e *Evaluator) popScope() error {
	if e.stackTop.scope.parent == nil {
		panic(fmt.Errorf("attempt to pop root scope"))
	}
	e.stackTop.scope = e.stackTop.scope.parent
	return nil
}

func (e *Evaluator) popFrame() error {
	if e.stackTop.parent == nil {
		panic(fmt.Errorf("attempt to pop root frame"))
	}
	e.stackTop = e.stackTop.parent
	return nil
}

func (e *Evaluator) getVariable(name string) (*scope, error) {
	scope := e.stackTop.scope
	for scope != nil {
		_, present := scope.bindings[name]
		if !present {
			scope = scope.parent
			continue
		}
		return scope, nil
	}

	// variable wasn't found
	// $fields don't get inferred values
	if strings.HasPrefix(name, "$") {
		return nil, fmt.Errorf("unknown variable %s", name)
	}
	// other variables get created in the current scope
	value := Value{Tag: ValueUnknown}
	e.stackTop.scope.bindings[name] = value
	return e.stackTop.scope, nil
}

func (e *Evaluator) setVariable(name string, value Value) {
	e.stackTop.scope.bindings[name] = value
}

func (e *Evaluator) setGlobal(name string, value Value) {
	top := e.stackTop
	for top.parent != nil {
		top = top.parent
	}
	top.scope.bindings[name] = value
}

func (e *Evaluator) getIdentifier(expr *ExprIdentifier) (Value, error) {
	if expr.token.Tag == Dollar {
		if e.ruleRoot == nil {
			return Value{}, e.error(expr.Token(), "unknown variable $")
		}
		return *e.ruleRoot, nil
	}

	lv, err := e.identifierLValue(expr)
	if err != nil {
		return Value{}, err
	}

	return lv.Get(), nil
}

func (e *Evaluator) identifierLValue(expr *ExprIdentifier) (LValue, error) {
	if expr.token.Tag == Dollar {
		if e.ruleRootSlot == nil {
			return nil, e.error(expr.Token(), "unknown variable $")
		}
		return e.ruleRootSlot, nil
	}

	scope, err := e.getVariable(expr.Ident)
	if err != nil {
		return nil, e.error(expr.Token(), err.Error())
	}
	return varLValue{scope, expr.Ident}, nil
}

func (e *Evaluator) evalString(str string) (Value, error) {
	buf := make([]byte, 0, len(str))
	for i := 0; i < len(str); i++ {
		b := str[i]
		if b != '\\' {
			buf = append(buf, b)
			continue
		}
		if i == len(str)-1 {
			return Value{}, fmt.Errorf("unexpected '\\' at end of string")
		}
		i++
		switch str[i] {
		case 'n':
			buf = append(buf, '\n')
		case '\\':
			buf = append(buf, '\\')
		case 't':
			buf = append(buf, '\t')
		default:
			return Value{}, fmt.Errorf("unknown escape char %q", str[i])
		}
	}
	s := string(buf)
	return NewString(s), nil
}

func (e *Evaluator) evalExpr(expr Expr) (Value, error) {
	if err := e.checkContext(); err != nil {
		return Value{}, err
	}

	switch exp := expr.(type) {
	case *ExprLiteral:
		switch exp.token.Tag {
		case Str, Ident:
			cell, err := e.evalString(exp.Literal)
			if err != nil {
				return Value{}, e.error(expr.Token(), err.Error())
			}
			return cell, nil
		case Regex:
			re, err := regexp.Compile(exp.Literal)
			if err != nil {
				return Value{}, e.error(expr.Token(), err.Error())
			}
			val := Value{
				Tag:    ValueRegex,
				Regexp: re,
			}
			return val, nil
		case Num:
			num, err := strconv.ParseFloat(exp.Literal, 64)
			if err != nil {
				return Value{}, e.error(expr.Token(), "could not parse number")
			}
			return NewValue(num), nil
		case True:
			return NewValue(true), nil
		case False:
			return NewValue(false), nil
		case Null:
			return NewValue(nil), nil
		default:
			panic(fmt.Errorf("unhandled literal type: %s", exp.token.Tag))
		}
	case *ExprUnary:
		return e.evalUnaryExpr(exp)
	case *ExprBinary:
		return e.evalBinaryExpr(exp)
	case *ExprIdentifier:
		return e.getIdentifier(exp)
	case *ExprCall:
		fn, err := e.evalExpr(exp.Func)
		if err != nil {
			return Value{}, err
		}

		args, err := e.evalExprList(exp.Args)
		if err != nil {
			return Value{}, err
		}
		argVals := make([]*Value, 0, len(args))
		for _, argVal := range args {
			argVals = append(argVals, &argVal)
		}

		result, err := e.callFunction(fn, argVals)
		if err != nil {
			switch err.(type) {
			case SyntaxError, RuntimeError:
				return Value{}, err
			default:
				return Value{}, e.error(exp.Token(), err.Error())
			}
		}

		return result, nil
	case *ExprArray:
		items, err := e.evalExprList(exp.Items)
		if err != nil {
			return Value{}, err
		}
		return NewValue(items), nil
	case *ExprMatch:
		value, err := e.evalExpr(exp.Value)
		if err != nil {
			return Value{}, err
		}

		for _, matchCase := range exp.Cases {
			isMatch := false

			var bindings map[string]Value
			isMatch, bindings, err = e.evalCaseMatch(value, matchCase.Exprs)
			if err != nil {
				return Value{}, err
			}

			if isMatch {
				e.pushScope()
				maps.Copy(e.stackTop.scope.bindings, bindings)

				switch body := matchCase.Body.(type) {
				case *StatementExpr:
					val, err := e.evalExpr(body.Expr)
					if err != nil {
						return Value{}, err
					}
					if err := e.popScope(); err != nil {
						return Value{}, err
					}
					return val, nil
				default:
					err := e.evalStatement(body)
					if err != nil {
						return Value{}, err
					}
				}

				if err := e.popScope(); err != nil {
					return Value{}, err
				}

				return NewValue(nil), nil
			}
		}
		return NewValue(nil), nil
	case *ExprObject:
		obj := NewObject()
		for _, kv := range exp.Items {
			value, err := e.evalExpr(kv.Value)
			if err != nil {
				return Value{}, err
			}

			obj.Obj.Set(kv.Key, value)
		}
		return obj, nil
	case *ExprFunction:
		fn := FnWithContext{
			Expr:  exp,
			Scope: e.stackTop.scope,
		}
		val := Value{
			Tag: ValueFn,
			Fn:  fn,
		}
		cell := val
		e.stackTop.scope.bindings[exp.Ident] = cell
		return cell, nil
	case *ExprAssign:
		val, err := e.evalExpr(exp.Value)
		if err != nil {
			return Value{}, err
		}
		cell, err := e.assignToTarget(exp.Target, val)
		if err != nil {
			return Value{}, err
		}
		return cell, nil
	}
	return Value{}, e.error(expr.Token(), "expected an expression")
}

func (e *Evaluator) memberLValue(parent Value, key Value) (LValue, error) {
	switch parent.Tag {
	case ValueObj:
		s := key.String()
		if _, present := parent.Obj.Get(s); !present {
			parent.Obj.Set(s, Value{Tag: ValueUnknown})
		}
		return objectLValue{parent.Obj, s}, nil
	case ValueArray:
		if key.Tag != ValueNum {
			return nil, fmt.Errorf("array indices must be numbers")
		}
		index, ok := getArrayIndex(*key.Num, len(parent.Array.Items))

		if !ok {
			// fill up to index with empty values
			if index < 0 {
				return nil, fmt.Errorf("index out of range")
			}

			// past the end of the array
			if index > 1024*1024 {
				return nil, fmt.Errorf("index too large to auto-fill array")
			}

			lastIndex := 0
			for i := len(parent.Array.Items); i <= index; i++ {
				lastIndex = i
				if i == index {
					parent.Array.Add(Value{Tag: ValueUnknown})
				} else {
					parent.Array.Add(NewValue(nil))
				}
			}
			index = lastIndex
		}

		return arrayLValue{parent.Array, index}, nil
	default:
		return nil, fmt.Errorf("invalid member assignment target")
	}
}

func (e *Evaluator) evalAssignTarget(target AssignTarget) (LValue, error) {
	var lv LValue
	var err error

	switch expr := target.Obj.(type) {
	case *ExprIdentifier:
		lv, err = e.identifierLValue(expr)
		if err != nil {
			return nil, err
		}
	default:
		return nil, e.error(expr.Token(), "invalid assignment target")
	}

	if len(target.Path) == 0 {
		return lv, nil
	}

	for i, seg := range target.Path {
		// resolve the key
		var key Value
		var tok Token

		if seg.Expr != nil {
			key, err = e.evalExpr(seg.Expr)
			if err != nil {
				return nil, err
			}
			tok = seg.Expr.Token()
		} else {
			key = NewString(seg.Field)
			tok = seg.token
		}

		parent := lv.Get()

		if parent.Tag == ValueUnknown {
			switch key.Tag {
			case ValueNum:
				lv.Set(NewArray())
			case ValueStr:
				lv.Set(NewObject())
			default:
				return nil, e.error(tok, fmt.Sprintf("cannot index with %s", key.Tag))
			}
		}

		childLv, err := e.memberLValue(lv.Get(), key)
		if err != nil {
			return nil, e.error(tok, err.Error())
		}

		if i == len(target.Path)-1 {
			return childLv, nil
		}

		lv = childLv
	}

	panic("unreachable")
}

func (e *Evaluator) assignToTarget(target AssignTarget, value Value) (Value, error) {
	lv, err := e.evalAssignTarget(target)
	if err != nil {
		return Value{}, err
	}

	lv.Set(value)
	return lv.Get(), nil
}

func (e *Evaluator) evalCaseMatch(value Value, exprs []Expr) (bool, map[string]Value, error) {
	for _, expr := range exprs {
		switch ex := expr.(type) {
		case *ExprLiteral:
			caseValue, err := e.evalExpr(expr)
			if err != nil {
				return false, nil, err
			}
			cmp, err := value.Compare(&caseValue)
			if err != nil {
				return false, nil, e.error(expr.Token(), err.Error())
			}
			if cmp == 0 {
				return true, nil, nil
			}
		case *ExprArray:
			if value.Tag != ValueArray {
				return false, nil, nil
			}

			array := value.Array
			if len(array.Items) != len(ex.Items) {
				return false, nil, nil
			}

			bindings := make(map[string]Value)

			for i, item := range array.Items {
				exprToMatch := ex.Items[i]
				match, newBindings, err := e.evalCaseMatch(*item, []Expr{exprToMatch})
				if err != nil {
					return false, nil, err
				}
				if !match {
					return false, nil, nil
				}
				maps.Copy(bindings, newBindings)
			}

			return true, bindings, nil
		case *ExprIdentifier:
			bindings := make(map[string]Value)
			bindings[ex.Ident] = value
			return true, bindings, nil
		default:
			return false, nil, e.error(expr.Token(), fmt.Sprintf("%s not supported in match expressions", expr))
		}
	}
	return false, nil, nil
}

func (e *Evaluator) callFunction(fn Value, args []*Value) (Value, error) {
	switch fn.Tag {
	case ValueNativeFn:
		result, err := fn.NativeFn(e, args, fn.Binding)
		if err != nil {
			return Value{}, err
		}
		if result != nil {
			return *result, nil
		}
		return NewValue(nil), nil
	case ValueFn:
		f := fn.Fn
		if err := e.pushFrame(f.Expr.Ident); err != nil {
			return Value{}, err
		}
		e.stackTop.scope.parent = f.Scope

		e.stackTop.scope.bindings[f.Expr.Ident] = fn

		for index, argName := range f.Expr.Args {
			if index > len(args)-1 {
				e.stackTop.scope.bindings[argName] = NewValue(nil)
			} else {
				e.stackTop.scope.bindings[argName] = *args[index]
			}
		}

		err := e.evalStatement(f.Expr.Body)

		var retVal *Value
		if err == errReturn {
			retVal = e.returnVal
		} else if err != nil {
			return Value{}, err
		} else {
			retVal = nil
		}

		if err := e.popFrame(); err != nil {
			return Value{}, err
		}

		if retVal != nil {
			return *retVal, nil
		}
		return NewValue(nil), nil
	default:
		return Value{}, fmt.Errorf("attempted to call a %s", fn.Tag)
	}
}

func (e *Evaluator) evalUnaryExpr(expr *ExprUnary) (Value, error) {
	val, err := e.evalExpr(expr.Expr)
	if err != nil {
		return Value{}, err
	}

	switch expr.OpToken.Tag {
	case Bang:
		return NewValue(!val.isTruthy()), nil
	case Plus:
		v := val.asFloat64()
		return NewValue(v), nil
	case Minus:
		v := val.asFloat64()
		return NewValue(-v), nil
	case PlusPlus, MinusMinus:
		v := val.asFloat64()
		var newValue Value

		switch expr.OpToken.Tag {
		case PlusPlus:
			newValue = NewValue(v + 1)
		case MinusMinus:
			newValue = NewValue(v - 1)
		}

		oldValue := val
		value, err := e.assignToTarget(expr.Target, newValue)
		if err != nil {
			return Value{}, err
		}

		if expr.Postfix {
			return NewValue(oldValue.asFloat64()), nil
		}
		return value, nil
	default:
		return Value{}, e.error(expr.OpToken, fmt.Sprintf("unknown operator %s", expr.OpToken.Tag))
	}
}

func (e *Evaluator) evalBinaryExpr(expr *ExprBinary) (Value, error) {
	left, err := e.evalExpr(expr.Left)
	if err != nil {
		return Value{}, err
	}

	// short circuiting operators
	switch expr.OpToken.Tag {
	case AmpAmp:
		if left.isTruthy() {
			right, err := e.evalExpr(expr.Right)
			if err != nil {
				return Value{}, err
			}
			if right.isTruthy() {
				return NewValue(true), nil
			}
		}
		return NewValue(false), nil
	case PipePipe:
		if left.isTruthy() {
			return NewValue(true), nil
		}

		right, err := e.evalExpr(expr.Right)
		if err != nil {
			return Value{}, err
		}

		if right.isTruthy() {
			return NewValue(true), nil
		}

		return NewValue(false), nil
	}

	// is special-case
	if expr.OpToken.Tag == Is {
		switch exp := expr.Right.(type) {
		case *ExprIdentifier:
			result := false

			switch exp.token.Tag {
			case Function:
				result = left.Tag == ValueFn
				return NewValue(result), nil
			case Null:
				result = left.Tag == ValueNil
				return NewValue(result), nil
			}

			s := exp.Ident
			switch s {
			case "string":
				result = left.Tag == ValueStr
			case "bool":
				result = left.Tag == ValueBool
			case "number":
				result = left.Tag == ValueNum
			case "array":
				result = left.Tag == ValueArray
			case "object":
				result = left.Tag == ValueObj
			case "regex":
				result = left.Tag == ValueRegex
			case "unknown":
				result = left.Tag == ValueUnknown
			}
			return NewValue(result), nil
		}

		return Value{}, e.error(expr.Right.Token(), "expected a type name")
	}

	rng, ok := expr.Right.(*ExprRange)
	if ok && expr.OpToken.Tag == LSquare {
		if left.Tag != ValueArray && left.Tag != ValueStr {
			return Value{}, e.error(rng.Token(), fmt.Sprintf("cannot slice a %s", left.Tag))
		}

		start := NewValue(0)
		if rng.Start != nil {
			start, err = e.evalExpr(rng.Start)
			if err != nil {
				return Value{}, err
			}
			if start.Tag != ValueNum {
				return Value{}, e.error(rng.Start.Token(), "slice start and end must be numbers")
			}
		}

		var end *Value
		if rng.End != nil {
			endVal, err := e.evalExpr(rng.End)
			end = &endVal
			if err != nil {
				return Value{}, err
			}
			if end.Tag != ValueNum {
				return Value{}, e.error(rng.End.Token(), "slice start and end must be numbers")
			}
		}

		if left.Tag == ValueArray {
			slice := NewArray()
			starti, startok := getArrayIndex(*start.Num, len(left.Array.Items))

			endi := len(left.Array.Items)
			endok := true
			if end != nil {
				endi, endok = getArrayIndex(*end.Num, len(left.Array.Items))
			}

			if !startok || !endok {
				return Value{}, e.error(rng.Token(), "index out of range")
			}

			if starti > endi {
				return Value{}, e.error(rng.Token(), "index out of range")
			}

			for i := starti; i < endi; i++ {
				value, _, err := left.GetMember(NewValue(i))
				if err != nil {
					return Value{}, err
				}
				slice.Array.Add(value)
			}
			return slice, nil
		}

		if left.Tag == ValueStr {
			starti, startok := getArrayIndex(*start.Num, len(*left.Str))
			endi := len(*left.Str)
			endok := true
			if end != nil {
				endi, endok = getArrayIndex(*end.Num, len(*left.Str))
			}

			if !startok || !endok {
				return Value{}, e.error(rng.Token(), "index out of range")
			}

			if starti > endi {
				return Value{}, e.error(rng.Token(), "index out of range")
			}

			str := (*left.Str)[starti:endi]
			return NewString(str), nil
		}
	}

	right, err := e.evalExpr(expr.Right)
	if err != nil {
		return Value{}, err
	}

	switch expr.OpToken.Tag {
	case LSquare, Dot:
		if left.Tag == ValueUnknown {
			if right.Tag == ValueNum {
				// if it's unknown and the rhs is a number, make it an array
				left = NewArray()
			} else {
				// otherwise make it an object
				left = NewObject()
			}
		}

		member, present, err := left.GetMember(right)
		if err != nil {
			return Value{}, e.error(expr.Left.Token(), err.Error())
		}

		if !present {
			return NewValue(nil), nil
		}
		member.Binding = &left

		return member, nil
	case LessThan, GreaterThan, EqualEqual, LessEqual, GreaterEqual, BangEqual:
		if left.Tag == ValueUnknown || right.Tag == ValueUnknown {
			// for unknown values, > and < are always true, == is always false
			switch expr.OpToken.Tag {
			case LessThan, GreaterThan:
				return NewValue(true), nil
			default:
				return NewValue(false), nil
			}
		}

		cmp, err := left.Compare(&right)
		if err != nil {
			return Value{}, e.error(expr.Left.Token(), err.Error())
		}
		switch expr.OpToken.Tag {
		case LessThan:
			return NewValue(cmp < 0), nil
		case GreaterThan:
			return NewValue(cmp > 0), nil
		case EqualEqual:
			return NewValue(cmp == 0), nil
		case BangEqual:
			v := NewValue(cmp == 0)
			return *v.Not(), nil
		case LessEqual:
			return NewValue(cmp <= 0), nil
		case GreaterEqual:
			return NewValue(cmp >= 0), nil
		default:
			panic("unhandled comparison operator")
		}
	case Plus, Minus, Multiply, Divide, Percent:
		if expr.OpToken.Tag == Plus && (left.Tag == ValueStr || right.Tag == ValueStr) {
			// string concat
			leftStr := left.String()
			rightStr := right.String()
			return NewValue(leftStr + rightStr), nil
		}

		leftNum := left.asFloat64()
		rightNum := right.asFloat64()
		switch expr.OpToken.Tag {
		case Plus:
			return NewValue(leftNum + rightNum), nil
		case Minus:
			return NewValue(leftNum - rightNum), nil
		case Multiply:
			return NewValue(leftNum * rightNum), nil
		case Divide:
			if leftNum == 0 || rightNum == 0 {
				return Value{}, e.error(expr.OpToken, "divide by zero")
			}
			return NewValue(leftNum / rightNum), nil
		case Percent:
			leftInt := int(leftNum)
			rightInt := int(rightNum)
			if leftInt == 0 || rightInt == 0 {
				return Value{}, e.error(expr.OpToken, "divide by zero")
			}
			return NewValue(leftInt % rightInt), nil
		default:
			panic("unhandled operator")
		}
	case Tilde, BangTilde:
		str := left.String()
		var re *regexp.Regexp
		switch right.Tag {
		case ValueStr:
			re, err = regexp.Compile(*right.Str)
			if err != nil {
				return Value{}, e.error(expr.Right.Token(), err.Error())
			}
		case ValueRegex:
			re = right.Regexp
		default:
			return Value{}, e.error(expr.Right.Token(), "a regex or a string must appear on the right hand side of ~")
		}

		var v Value
		if re.MatchString(str) {
			v = NewValue(true)
		} else {
			v = NewValue(false)
		}

		if expr.OpToken.Tag == BangTilde {
			return *v.Not(), nil
		}
		return v, nil
	default:
		return Value{}, e.error(expr.OpToken, fmt.Sprintf("unknown operator %s", expr.OpToken.Tag))
	}
}

func (e *Evaluator) evalExprList(exprs []Expr) ([]Value, error) {
	evaledExprs := make([]Value, 0, len(exprs))
	for _, expr := range exprs {
		v, err := e.evalExpr(expr)
		if err != nil {
			return evaledExprs, err
		}

		evaledExprs = append(evaledExprs, v)
	}
	return evaledExprs, nil
}

func (e *Evaluator) evalStatement(stmt Statement) error {
	if err := e.checkContext(); err != nil {
		return err
	}

	switch st := stmt.(type) {
	case *StatementBlock:
		for _, s := range st.Body {
			err := e.evalStatement(s)
			if err != nil {
				return err
			}
		}
		return nil
	case *StatementPrint:
		args, err := e.evalExprList(st.Args)
		if err != nil {
			return err
		}

		if len(args) == 0 {
			if e.ruleRoot == nil {
				return e.error(stmt.Token(), "unknown variable $")
			}
			val := e.ruleRoot
			fmt.Fprintln(e.stdout, val.PrettyString(false))
			return nil
		}

		for i, value := range args {
			if i > 0 {
				fmt.Fprint(e.stdout, " ")
			}
			fmt.Fprintf(e.stdout, "%s", value.PrettyString(false))
		}
		fmt.Fprint(e.stdout, "\n")
	case *StatementExpr:
		_, err := e.evalExpr(st.Expr)
		if err != nil {
			return err
		}
		return nil
	case *StatementReturn:
		if st.Expr != nil {
			value, err := e.evalExpr(st.Expr)
			if err != nil {
				return err
			}
			e.returnVal = &value
		} else {
			e.returnVal = nil
		}
		return errReturn
	case *StatementIf:
		value, err := e.evalExpr(st.Expr)
		if err != nil {
			return err
		}
		if value.isTruthy() {
			return e.evalStatement(st.Body)
		} else if st.ElseBody != nil {
			return e.evalStatement(st.ElseBody)
		}
	case *StatementWhile:
		loopCount := 0
		for {
			value, err := e.evalExpr(st.Expr)
			if err != nil {
				return err
			}
			if value.isTruthy() {
				err := e.evalStatement(st.Body)
				if err == errBreak {
					break
				} else if err != nil && err != errContinue {
					return err
				}
			} else {
				break
			}

			if e.fuzzing {
				if loopCount > fuzzingLoopLimit {
					return e.error(st.Token(), "fuzz test loop limit")
				}
			}
			loopCount++
		}
	case *StatementFor:
		e.evalExpr(st.PreExpr)
		loopCount := 0
		for {
			value, err := e.evalExpr(st.Expr)
			if err != nil {
				return err
			}
			if value.isTruthy() {
				err := e.evalStatement(st.Body)
				if err == errBreak {
					break
				} else if err != nil && err != errContinue {
					return err
				}

				_, err = e.evalExpr(st.PostExpr)
				if err != nil {
					return err
				}
			} else {
				break
			}

			if e.fuzzing {
				if loopCount > fuzzingLoopLimit {
					return e.error(st.Token(), "fuzz test loop limit")
				}
				loopCount++
			}
		}
	case *StatementForIn:
		// local, err := e.getVariable(st.Ident.Ident)
		// if err != nil {
		// 	return e.error(st.Token(), err.Error())
		// }

		// var indexLocal Value
		// if st.IndexIdent != nil {
		// 	indexLocal, err = e.getVariable(st.IndexIdent.Ident)
		// 	if err != nil {
		// 		return e.error(st.Token(), err.Error())
		// 	}
		// }

		iterable, err := e.evalExpr(st.Iterable)
		if err != nil {
			return err
		}

		switch iterable.Tag {
		case ValueArray:
			for index, item := range iterable.Array.Items {
				if st.IndexIdent != nil {
					e.setVariable(st.IndexIdent.Ident, NewValue(index))
				}

				e.setVariable(st.Ident.Ident, *item)

				err := e.evalStatement(st.Body)
				if err == errBreak {
					break
				} else if err != nil && err != errContinue {
					return err
				}
			}
		case ValueObj:
			for _, k := range iterable.Obj.Keys {
				v, _ := iterable.Obj.Get(k)
				if st.IndexIdent != nil {
					e.setVariable(st.IndexIdent.Ident, *v)
				}

				e.setVariable(st.Ident.Ident, NewValue(k))

				err := e.evalStatement(st.Body)
				if err == errBreak {
					break
				} else if err != nil && err != errContinue {
					return err
				}
			}
		case ValueStr:
			for index, c := range *iterable.Str {
				if st.IndexIdent != nil {
					e.setVariable(st.IndexIdent.Ident, NewValue(index))
				}

				e.setVariable(st.Ident.Ident, NewValue(string(c)))

				err := e.evalStatement(st.Body)
				if err == errBreak {
					break
				} else if err != nil && err != errContinue {
					return err
				}
			}
		default:
			return e.error(st.Iterable.Token(), fmt.Sprintf("%s is not iterable", iterable.Tag))
		}
	case *StatementBreak:
		return errBreak
	case *StatementContinue:
		return errContinue
	case *StatementNext:
		return errNext
	case *StatementExit:
		return errExit
	case *StatementLet:
		ident := st.Ident.Ident
		_, present := e.stackTop.scope.bindings[ident]
		if present {
			return e.error(st.Token(), fmt.Sprintf("variable '%s' already declared", ident))
		}

		value, err := e.evalExpr(st.Value)
		if err != nil {
			return err
		}

		e.stackTop.scope.bindings[ident] = value
		return nil
	default:
		return e.error(st.Token(), fmt.Sprintf("expected a statement but found %T", st))
	}
	return nil
}

func (e *Evaluator) evalRules(rules []*Rule) error {
	for _, rule := range rules {
		match := true
		if rule.Pattern != nil {
			value, err := e.evalExpr(rule.Pattern)
			if err != nil {
				return err
			}
			match = value.isTruthy()
		}

		if !match {
			continue
		}

		err := e.evalStatement(rule.Body)
		if err == errNext {
			return nil
		}
		if err != nil {
			return err
		}
	}
	return nil
}

func (e *Evaluator) evalPatternRules(patternRules []*Rule) error {
	if e.root == nil {
		return nil
	}

	switch e.root.Tag {
	case ValueArray:
		for i, item := range e.root.Array.Items {
			e.stackTop.scope.bindings["$index"] = NewValue(i)
			e.setRuleRoot(item, arrayLValue{e.root.Array, i})
			if err := e.evalRules(patternRules); err != nil {
				return err
			}
		}
	default:
		e.setRuleRoot(e.root, nil)
		if err := e.evalRules(patternRules); err != nil {
			return err
		}
	}

	return nil
}

func (e *Evaluator) GetPrettyRootJson() (string, error) {
	bytes, err := json.MarshalIndent(e.root, "", "  ")
	if err != nil {
		return "", err
	}
	return string(bytes), nil
}

func (e *Evaluator) GetUglyRootJson() (string, error) {
	bytes, err := json.Marshal(e.root)
	if err != nil {
		return "", err
	}
	return string(bytes), nil
}

func (e *Evaluator) checkContext() error {
	if e.ctx == nil {
		return nil
	}

	if err := e.ctx.Err(); err != nil {
		return RuntimeError{Message: err.Error()}
	}

	return nil
}

func (e *Evaluator) setRuleRoot(value *Value, slot LValue) {
	e.ruleRoot = value
	e.ruleRootSlot = &rootLValue{e, slot}
}

func (e *Evaluator) clearRuleRoot() {
	e.ruleRoot = nil
	e.ruleRootSlot = nil
}

func (e *Evaluator) forEachRootValue(files []InputFile, rootSelectors []string, fn func(*Value) error) error {
	parsedRootSelectors := make([]Expr, len(rootSelectors))

	for i, rootSelector := range rootSelectors {
		lex := NewLexer(rootSelector)
		parser := NewParser(&lex)
		expr, err := parser.ParseExpression()
		if err != nil {
			return err
		}
		parsedRootSelectors[i] = expr
	}

	for _, file := range files {
		// for each json value
		jp := newJsonParser(file.NewReader())
		for {
			rootValue, err := jp.next()
			if err != nil {
				if err == io.EOF {
					break
				}
				return JsonError{err.Error(), file.Name()}
			}

			e.setGlobal("$file", NewValue(file.Name()))

			// find the root value(s)
			rootValues := make([]*Value, 0)
			if len(parsedRootSelectors) > 0 {
				for _, expr := range parsedRootSelectors {
					e.root = &rootValue
					e.setRuleRoot(e.root, nil)
					value, err := e.evalExpr(expr)
					if err != nil && err != errExit {
						return err
					}
					rootValues = append(rootValues, &value)
				}
			} else {
				rootValues = append(rootValues, &rootValue)
			}

			for _, rootValue := range rootValues {
				if err = fn(rootValue); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

func (e *Evaluator) RunInBeginFileContext(files []InputFile, rootSelectors []string, fn func() error) error {
	return e.forEachRootValue(files, rootSelectors, func(rootValue *Value) error {
		e.root = rootValue
		e.setRuleRoot(e.root, nil)
		if err := fn(); err != nil {
			return err
		}
		return nil
	})
}

func (e *Evaluator) RunProgram(prog Program, files []InputFile, rootSelectors []string) error {
	e.prog = prog
	e.readRules()
	e.addProgramFunctions()
	_, err := evalProgramInternal(e, files, rootSelectors)
	return err
}

func (e *Evaluator) EvalExpr(expr Expr) (Value, error) {
	return e.evalExpr(expr)
}

func (e *Evaluator) EvalStatement(stmt Statement) error {
	return e.evalStatement(stmt)
}

func EvalProgram(progSrc string, files []InputFile, rootSelectors []string, stdout io.Writer, fuzzing bool) (*Evaluator, error) {
	lex := NewLexer(progSrc)
	parser := NewParser(&lex)
	prog, err := parser.Parse()
	if err != nil {
		return nil, err
	}
	ev := NewEvaluator(prog, stdout)
	ev.fuzzing = fuzzing
	return evalProgramInternal(&ev, files, rootSelectors)
}

func EvalProgramContext(progSrc string, files []InputFile, rootSelectors []string, stdout io.Writer, fuzzing bool, ctx context.Context) (*Evaluator, error) {
	lex := NewLexer(progSrc)
	parser := NewParser(&lex)
	prog, err := parser.Parse()
	if err != nil {
		return nil, err
	}
	ev := NewEvaluator(prog, stdout)
	ev.ctx = ctx
	ev.fuzzing = fuzzing
	return evalProgramInternal(&ev, files, rootSelectors)
}

func evalProgramInternal(ev *Evaluator, files []InputFile, rootSelectors []string) (*Evaluator, error) {
	// begin rules
	for _, rule := range ev.beginRules {
		ev.clearRuleRoot()
		if err := ev.evalStatement(rule.Body); err != nil {
			if err == errExit {
				return ev, nil
			}
			return ev, err
		}
	}

	// for each file, run the pattern rules
	err := ev.forEachRootValue(files, rootSelectors, func(rootValue *Value) error {
		// run the begin file rules
		ev.setRuleRoot(rootValue, nil)
		for _, rule := range ev.beginFileRules {
			if err := ev.evalStatement(rule.Body); err != nil {
				if err == errExit {
					return nil
				}
				return err
			}
		}
		modifiedRoot := ev.ruleRoot

		// run the rules
		ev.root = modifiedRoot
		if err := ev.evalPatternRules(ev.patternRules); err != nil {
			if err == errExit {
				return nil
			}
			return err
		}

		// run the end file rules
		ev.setRuleRoot(modifiedRoot, nil)
		for _, rule := range ev.endFileRules {
			if err := ev.evalStatement(rule.Body); err != nil {
				if err == errExit {
					return nil
				}
				return err
			}
		}

		return nil
	})

	if err != nil {
		return ev, err
	}

	// end rules
	for _, rule := range ev.endRules {
		ev.clearRuleRoot()
		if err := ev.evalStatement(rule.Body); err != nil {
			if err == errExit {
				return ev, nil
			}
			return ev, err
		}
	}

	return ev, nil
}
