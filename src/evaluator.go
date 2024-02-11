package lang

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"regexp"
	"strconv"
	"strings"
)

type stackFrame struct {
	name   string
	locals map[string]*Cell
	parent *stackFrame
}

type Evaluator struct {
	prog         Program
	lexer        *Lexer
	stdout       io.Writer
	root         *Cell
	ruleRoot     *Cell
	stackTop     *stackFrame
	returnVal    *Value
	beginRules   []*Rule
	patternRules []*Rule
	endRules     []*Rule
}

var (
	errContinue = errors.New("continue")
	errBreak    = errors.New("break")
	errReturn   = errors.New("return")
	errNext     = errors.New("next")
	errExit     = errors.New("exit")
)

func NewEvaluator(prog Program, lexer *Lexer, stdout io.Writer) Evaluator {
	e := Evaluator{
		prog:   prog,
		lexer:  lexer,
		stdout: stdout,
	}
	e.readRules()
	e.pushFrame("<root>")
	addRuntimeFunctions(&e)
	e.addProgramFunctions()
	return e
}

func (e *Evaluator) readRules() {
	e.beginRules = make([]*Rule, 0)
	e.endRules = make([]*Rule, 0)
	e.patternRules = make([]*Rule, 0)

	for _, rule := range e.prog.Rules {
		r := rule
		switch rule.Kind {
		case BeginRule:
			e.beginRules = append(e.beginRules, &r)
		case EndRule:
			e.endRules = append(e.endRules, &r)
		case PatternRule:
			e.patternRules = append(e.patternRules, &r)
		}
	}
}

func (e *Evaluator) addProgramFunctions() {
	for _, fn := range e.prog.Functions {
		f := fn
		val := Value{
			Tag: ValueFn,
			Fn:  &f,
		}
		name := e.lexer.GetString(&fn.ident)
		e.stackTop.locals[name] = NewCell(val)
	}
}

func (e *Evaluator) print(str string) {
	fmt.Fprint(e.stdout, str)
}

func (e *Evaluator) error(token Token, msg string) RuntimeError {
	srcLine, line, col := e.lexer.GetLineAndCol(token.Pos)
	return RuntimeError{
		Message: msg,
		Line:    line,
		Col:     col,
		SrcLine: srcLine,
	}
}

func (e *Evaluator) pushFrame(name string) {
	frame := stackFrame{
		name:   name,
		locals: make(map[string]*Cell),
		parent: e.stackTop,
	}
	e.stackTop = &frame
}

func (e *Evaluator) popFrame() error {
	if e.stackTop.parent == nil {
		panic(fmt.Errorf("attempt to pop root frame"))
	}
	e.stackTop = e.stackTop.parent
	return nil
}

func (e *Evaluator) getVariable(name string) (*Cell, error) {
	frame := e.stackTop
	for frame != nil {
		cell, present := frame.locals[name]
		if !present {
			frame = frame.parent
			continue
		}
		return cell, nil
	}

	// variable wasn't found
	// $fields don't get inferred values
	if strings.HasPrefix(name, "$") {
		return nil, fmt.Errorf("unknown variable %s", name)
	}
	// other variables get created in the current scope
	cell := NewCell(Value{Tag: ValueUnknown})
	e.stackTop.locals[name] = cell
	return cell, nil
}

func (e *Evaluator) setGlobal(name string, cell *Cell) {
	top := e.stackTop
	for top.parent != nil {
		top = top.parent
	}
	top.locals[name] = cell
}

func (e *Evaluator) getIdentifier(expr *ExprIdentifier) (*Cell, error) {
	if expr.token.Tag == Dollar {
		if e.ruleRoot != nil {
			return e.ruleRoot, nil
		} else {
			return nil, e.error(expr.Token(), "unknown variable $")
		}
	} else {
		ident := e.lexer.GetString(&expr.token)
		local, err := e.getVariable(ident)
		if err != nil {
			return nil, e.error(expr.Token(), err.Error())
		}
		return local, nil
	}
}

func (e *Evaluator) evalString(str string) (*Cell, error) {
	buf := make([]byte, 0, len(str))
	for i := 0; i < len(str); i++ {
		b := str[i]
		if b != '\\' {
			buf = append(buf, b)
			continue
		}
		if i == len(str)-1 {
			return nil, fmt.Errorf("unexpected '\\' at end of string")
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
			return nil, fmt.Errorf("unknown escape char %q", str[i])
		}
	}
	s := string(buf)
	return NewCell(NewString(s)), nil
}

func (e *Evaluator) evalExpr(expr Expr) (*Cell, error) {
	switch exp := expr.(type) {
	case *ExprLiteral:
		switch exp.token.Tag {
		case Str, Ident:
			str := e.lexer.GetString(&exp.token)
			cell, err := e.evalString(str)
			if err != nil {
				return nil, e.error(expr.Token(), err.Error())
			}
			return cell, nil
		case Regex:
			str := e.lexer.GetString(&exp.token)
			val := Value{
				Tag: ValueRegex,
				Str: &str,
			}
			return NewCell(val), nil
		case Num:
			numStr := e.lexer.GetString(&exp.token)
			num, err := strconv.ParseInt(numStr, 10, 64)
			if err != nil {
				return nil, e.error(expr.Token(), "could not parse number")
			}
			f := float64(num)
			return NewCell(Value{
				Tag: ValueNum,
				Num: &f,
			}), nil
		case True:
			return NewCell(NewValue(true)), nil
		case False:
			return NewCell(NewValue(false)), nil
		case Null:
			return NewCell(NewValue(nil)), nil
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
			return nil, err
		}

		args, err := e.evalExprList(exp.Args)
		if err != nil {
			return nil, err
		}
		argVals := make([]*Value, 0, len(args))
		for _, argCell := range args {
			argVals = append(argVals, &argCell.Value)
		}

		result, err := e.callFunction(exp, fn, argVals)
		if err != nil {
			return nil, err
		}
		return result, nil
	case *ExprArray:
		items, err := e.evalExprList(exp.Items)
		if err != nil {
			return nil, err
		}
		return NewCell(NewValue(items)), nil
	case *ExprMatch:
		value, err := e.evalExpr(exp.Value)
		if err != nil {
			return nil, err
		}

		for _, matchCase := range exp.Cases {
			isMatch := false

			var bindings map[string]*Cell
			isMatch, bindings, err = e.evalCaseMatch(value, matchCase.Exprs)
			if err != nil {
				return nil, err
			}

			if isMatch {
				e.pushFrame("<match>")
				for k, v := range bindings {
					e.stackTop.locals[k] = v
				}

				switch body := matchCase.Body.(type) {
				case *StatementExpr:
					val, err := e.evalExpr(body.Expr)
					if err != nil {
						return nil, err
					}
					return val, nil
				default:
					err := e.evalStatement(body)
					if err != nil {
						return nil, err
					}
				}

				if err := e.popFrame(); err != nil {
					return nil, err
				}

				return nil, nil
			}
		}
		return NewCell(NewValue(nil)), nil
	case *ExprObject:
		obj := NewObject()
		for _, kv := range exp.Items {
			value, err := e.evalExpr(kv.Value)
			if err != nil {
				return nil, err
			}
			(*obj.Obj)[kv.Key] = value
		}
		return NewCell(obj), nil
	default:
		return nil, e.error(exp.Token(), "expected an expression")
	}
}

func (e *Evaluator) evalCaseMatch(value *Cell, exprs []Expr) (bool, map[string]*Cell, error) {
	for _, expr := range exprs {
		switch ex := expr.(type) {
		case *ExprLiteral:
			caseValue, err := e.evalExpr(expr)
			if err != nil {
				return false, nil, err
			}
			cmp, err := value.Value.Compare(&caseValue.Value)
			if err != nil {
				return false, nil, err
			}
			if cmp == 0 {
				return true, nil, nil
			}
		case *ExprArray:
			if value.Value.Tag != ValueArray {
				return false, nil, nil
			}

			array := value.Value.Array
			if len(array) != len(ex.Items) {
				return false, nil, nil
			}

			bindings := make(map[string]*Cell)

			for i, item := range array {
				exprToMatch := ex.Items[i]
				match, newBindings, err := e.evalCaseMatch(item, []Expr{exprToMatch})
				if err != nil {
					return false, nil, err
				}
				if !match {
					return false, nil, nil
				}
				for k, v := range newBindings {
					bindings[k] = v
				}
			}

			return true, bindings, nil
		case *ExprIdentifier:
			bindings := make(map[string]*Cell)
			ident := e.lexer.GetString(&ex.token)
			bindings[ident] = value
			return true, bindings, nil
		default:
			return false, nil, e.error(expr.Token(), fmt.Sprintf("%s not supported in match expressions", expr))
		}
	}
	return false, nil, nil
}

func (e *Evaluator) callFunction(exp *ExprCall, fn *Cell, args []*Value) (*Cell, error) {
	switch fn.Value.Tag {
	case ValueNativeFn:
		result, err := fn.Value.NativeFn(e, args, fn.Value.Binding)
		if err != nil {
			return nil, e.error(exp.Token(), err.Error())
		}
		if result != nil {
			return NewCell(*result), nil
		}
		return NewCell(NewValue(nil)), nil
	case ValueFn:
		f := fn.Value.Fn
		name := e.lexer.GetString(&f.ident)
		e.pushFrame(name)
		for index, argName := range f.Args {
			e.stackTop.locals[argName] = NewCell(*args[index])
		}

		err := e.evalStatement(f.Body)
		var retVal *Value
		if err == errReturn {
			retVal = e.returnVal
		} else if err != nil {
			return nil, err
		} else {
			retVal = nil
		}

		if err := e.popFrame(); err != nil {
			return nil, err
		}

		if retVal != nil {
			return NewCell(*retVal), nil
		}
		return nil, nil
	default:
		return nil, e.error(exp.Token(), fmt.Sprintf("attempted to call a %s", fn.Value.Tag))
	}
}

func (e *Evaluator) evalUnaryExpr(expr *ExprUnary) (*Cell, error) {
	val, err := e.evalExpr(expr.Expr)
	if err != nil {
		return nil, err
	}

	switch expr.OpToken.Tag {
	case Bang:
		return NewCell(NewValue(!val.Value.isTruthy())), nil
	case Plus:
		v := val.Value.asFloat64()
		return NewCell(NewValue(v)), nil
	case Minus:
		v := val.Value.asFloat64()
		return NewCell(NewValue(-v)), nil
	case PlusPlus, MinusMinus:
		v := val.Value.asFloat64()

		switch expr.OpToken.Tag {
		case PlusPlus:
			val.Value = NewValue(v + 1)
		case MinusMinus:
			val.Value = NewValue(v - 1)
		}

		if expr.Postfix {
			return NewCell(NewValue(v)), nil
		}
		return NewCell(val.Value), nil
	default:
		return nil, e.error(expr.OpToken, fmt.Sprintf("unknown operator %s", expr.OpToken.Tag))
	}
}

func (e *Evaluator) evalBinaryExpr(expr *ExprBinary) (*Cell, error) {
	left, err := e.evalExpr(expr.Left)
	if err != nil {
		return nil, err
	}

	// short circuiting operators
	switch expr.OpToken.Tag {
	case AmpAmp:
		if left.Value.isTruthy() {
			right, err := e.evalExpr(expr.Right)
			if err != nil {
				return nil, err
			}
			if right.Value.isTruthy() {
				return NewCell(NewValue(true)), nil
			}
		}
		return NewCell(NewValue(false)), nil
	case PipePipe:
		if left.Value.isTruthy() {
			return NewCell(NewValue(true)), nil
		}

		right, err := e.evalExpr(expr.Right)
		if err != nil {
			return nil, err
		}

		if right.Value.isTruthy() {
			return NewCell(NewValue(true)), nil
		}

		return NewCell(NewValue(false)), nil
	}

	right, err := e.evalExpr(expr.Right)
	if err != nil {
		return nil, err
	}

	switch expr.OpToken.Tag {
	case LSquare, Dot:
		if left.Value.Tag == ValueUnknown {
			if right.Value.Tag == ValueNum {
				// if it's unknown and the rhs is a number, make it an array
				left.Value = NewArray()
			} else {
				// otherwise make it an object
				left.Value = NewObject()
			}
		}

		member, err := left.Value.GetMember(right.Value)
		if err != nil {
			return nil, e.error(expr.Left.Token(), err.Error())
		}

		if member == nil {
			// speculatively create members
			// see createSpeculativeObjects
			memberVal := NewValue(nil)
			rightStr := right.Value.String()
			memberVal.Str = &rightStr
			memberVal.ParentObj = &left.Value
			return NewCell(memberVal), nil
		}
		member.Value.Binding = &left.Value

		return member, nil
	case LessThan, GreaterThan, EqualEqual, LessEqual, GreaterEqual, BangEqual:
		if left.Value.Tag == ValueUnknown || right.Value.Tag == ValueUnknown {
			// for unknown values, > and < are always true, == is always false
			switch expr.OpToken.Tag {
			case LessThan, GreaterThan:
				return NewCell(NewValue(true)), nil
			default:
				return NewCell(NewValue(false)), nil
			}
		}

		cmp, err := left.Value.Compare(&right.Value)
		if err != nil {
			return nil, e.error(expr.Left.Token(), err.Error())
		}
		switch expr.OpToken.Tag {
		case LessThan:
			return NewCell(NewValue(cmp < 0)), nil
		case GreaterThan:
			return NewCell(NewValue(cmp > 0)), nil
		case EqualEqual:
			return NewCell(NewValue(cmp == 0)), nil
		case BangEqual:
			v := NewValue(cmp == 0)
			return NewCell(*v.Not()), nil
		case LessEqual:
			return NewCell(NewValue(cmp <= 0)), nil
		case GreaterEqual:
			return NewCell(NewValue(cmp >= 0)), nil
		default:
			panic("unhandled comparison operator")
		}
	case Plus, Minus, Multiply, Divide, Percent:
		if expr.OpToken.Tag == Plus && (left.Value.Tag == ValueStr || right.Value.Tag == ValueStr) {
			// string concat
			leftStr := left.Value.String()
			rightStr := right.Value.String()
			return NewCell(NewValue(leftStr + rightStr)), nil
		}

		leftNum := left.Value.asFloat64()
		rightNum := right.Value.asFloat64()
		switch expr.OpToken.Tag {
		case Plus:
			return NewCell(NewValue(leftNum + rightNum)), nil
		case Minus:
			return NewCell(NewValue(leftNum - rightNum)), nil
		case Multiply:
			return NewCell(NewValue(leftNum * rightNum)), nil
		case Divide:
			if leftNum == 0 || rightNum == 0 {
				return nil, e.error(expr.OpToken, "divide by zero")
			}
			return NewCell(NewValue(leftNum / rightNum)), nil
		case Percent:
			leftInt := int(leftNum)
			rightInt := int(rightNum)
			if leftInt == 0 || rightInt == 0 {
				return nil, e.error(expr.OpToken, "divide by zero")
			}
			return NewCell(NewValue(leftInt % rightInt)), nil
		default:
			panic("unhandled operator")
		}
	case Tilde, BangTilde:
		str := left.Value.String()
		var regex string
		switch right.Value.Tag {
		case ValueStr:
			regex = *right.Value.Str
		case ValueRegex:
			regex = *right.Value.Str
		default:
			return nil, e.error(expr.Right.Token(), "a regex or a string must appear on the right hand side of ~")
		}

		re, err := regexp.Compile(regex)
		if err != nil {
			return nil, e.error(expr.Right.Token(), err.Error())
		}

		var v Value
		if re.MatchString(str) {
			v = NewValue(true)
		} else {
			v = NewValue(false)
		}

		if expr.OpToken.Tag == BangTilde {
			return NewCell(*v.Not()), nil
		}
		return NewCell(v), nil
	case Equal:
		return e.evalAssignment(expr, left, right)
	default:
		return nil, e.error(expr.OpToken, fmt.Sprintf("unknown operator %s", expr.OpToken.Tag))
	}
}

func (e *Evaluator) createSpeculativeObjects(specObj *Cell) (*Cell, error) {
	// Speculative objects are how jqawk implements two features:
	//
	// 1. Optional chaining, where "a.b.c.d" should evaluate to nil if a is an
	//  	object but has no "b" property
	//
	// 2. Implicit member creation, where "a.b.c.d = 4" should create all the
	// 		intervening objects
	//
	// To support both, b, c, and d are created as speculative objects, a special
	// class of nil value with ParentObj and Str properties. If a speculative
	// object is assigned to, we create it. If not, it's just a nil.
	//
	// ParentObj points to the object this value should be set on, Str is the key.
	// ParentObj could also be a speculative object, so this function recursively
	// create parents.
	parent := specObj.Value.ParentObj

	if parent.Tag == ValueNil && parent.ParentObj == nil {
		return nil, fmt.Errorf("could not create this object")
	}

	var objToSet *Value
	if parent.Tag == ValueNil {
		newParent, err := e.createSpeculativeObjects(NewCell(*parent))
		if err != nil {
			return nil, err
		}
		newObj := NewObject()
		newParent.Value = newObj
		objToSet = &newParent.Value
	} else {
		objToSet = parent
	}

	cell, err := objToSet.SetMember(NewString(*specObj.Value.Str), specObj)
	if err != nil {
		return nil, err
	}

	return cell, nil
}

func (e *Evaluator) evalAssignment(expr *ExprBinary, left *Cell, right *Cell) (*Cell, error) {
	if left.Value.Tag == ValueNil && left.Value.ParentObj != nil {
		// speculative object creation
		var err error
		left, err = e.createSpeculativeObjects(left)
		if err != nil {
			return nil, e.error(expr.OpToken, err.Error())
		}
	}

	switch right.Value.Tag {
	// copy
	case ValueNum:
		n := *right.Value.Num
		left.Value = Value{
			Tag: ValueNum,
			Num: &n,
		}
	case ValueBool:
		b := *right.Value.Bool
		left.Value = Value{
			Tag:  ValueBool,
			Bool: &b,
		}
	case ValueNil:
		left.Value = NewValue(nil)
	case ValueStr:
		s := *right.Value.Str
		left.Value = NewString(s)
	case ValueRegex:
		r := *right.Value.Str
		val := Value{
			Tag: ValueRegex,
			Str: &r,
		}
		left.Value = val

	// reference
	case ValueArray, ValueObj, ValueUnknown:
		left.Value = right.Value

	default:
		return nil, e.error(expr.OpToken, "invalid assignment")
	}
	return left, nil
}

func (e *Evaluator) evalExprList(exprs []Expr) ([]*Cell, error) {
	evaledExprs := make([]*Cell, 0, len(exprs))
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
			fmt.Fprintln(e.stdout, e.ruleRoot.Value.PrettyString(false))
			return nil
		}

		for i, cell := range args {
			if i > 0 {
				fmt.Fprint(e.stdout, " ")
			}
			if cell == nil {
				fmt.Fprint(e.stdout, "null")
			} else {
				fmt.Fprintf(e.stdout, "%s", cell.Value.PrettyString(false))
			}
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
			cell, err := e.evalExpr(st.Expr)
			if err != nil {
				return err
			}
			e.returnVal = &cell.Value
		} else {
			e.returnVal = nil
		}
		return errReturn
	case *StatementIf:
		cell, err := e.evalExpr(st.Expr)
		if err != nil {
			return err
		}
		if cell.Value.isTruthy() {
			return e.evalStatement(st.Body)
		} else if st.ElseBody != nil {
			return e.evalStatement(st.ElseBody)
		}
	case *StatementWhile:
		for {
			cell, err := e.evalExpr(st.Expr)
			if err != nil {
				return err
			}
			if cell.Value.isTruthy() {
				err := e.evalStatement(st.Body)
				if err == errBreak {
					break
				} else if err != nil && err != errContinue {
					return err
				}
			} else {
				break
			}
		}
	case *StatementFor:
		e.evalExpr(st.PreExpr)
		for {
			cell, err := e.evalExpr(st.Expr)
			if err != nil {
				return err
			}
			if cell.Value.isTruthy() {
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
		}
	case *StatementForIn:
		ident := e.lexer.GetString(&st.Ident.token)
		local, err := e.getVariable(ident)
		if err != nil {
			return err
		}

		var indexLocal *Cell
		if st.IndexIdent != nil {
			indexIdent := e.lexer.GetString(&st.IndexIdent.token)
			indexLocal, err = e.getVariable(indexIdent)
			if err != nil {
				return err
			}
		}

		iterable, err := e.evalExpr(st.Iterable)
		if err != nil {
			return err
		}

		switch iterable.Value.Tag {
		case ValueArray:
			for index, item := range iterable.Value.Array {
				if indexLocal != nil {
					indexLocal.Value = NewValue(index)
				}
				local.Value = item.Value
				err := e.evalStatement(st.Body)
				if err == errBreak {
					break
				} else if err != nil && err != errContinue {
					return err
				}
			}
		case ValueObj:
			for k, v := range *iterable.Value.Obj {
				if indexLocal != nil {
					indexLocal.Value = v.Value
				}
				local.Value = NewValue(k)
				err := e.evalStatement(st.Body)
				if err == errBreak {
					break
				} else if err != nil && err != errContinue {
					return err
				}
			}
		case ValueStr:
			for index, c := range *iterable.Value.Str {
				if indexLocal != nil {
					indexLocal.Value = NewValue(index)
				}
				local.Value = NewString(string(c))
				err := e.evalStatement(st.Body)
				if err == errBreak {
					break
				} else if err != nil && err != errContinue {
					return err
				}
			}
		default:
			return e.error(st.Iterable.Token(), fmt.Sprintf("%s is not iterable", iterable.Value.Tag))
		}
	case *StatementBreak:
		return errBreak
	case *StatementContinue:
		return errContinue
	case *StatementNext:
		return errNext
	case *StatementExit:
		return errExit
	default:
		return e.error(st.Token(), fmt.Sprintf("expected a statement but found %T", st))
	}
	return nil
}

func (e *Evaluator) evalRules(rules []*Rule) error {
	for _, rule := range rules {
		match := true
		if rule.Pattern != nil {
			cell, err := e.evalExpr(rule.Pattern)
			if err != nil {
				return err
			}
			match = cell.Value.isTruthy()
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

	switch e.root.Value.Tag {
	case ValueArray:
		for i, item := range e.root.Value.Array {
			e.ruleRoot = item
			e.stackTop.locals["$index"] = NewCell(NewValue(i))
			if err := e.evalRules(patternRules); err != nil {
				return err
			}
		}
	case ValueObj:
		e.ruleRoot = e.root
		if err := e.evalRules(patternRules); err != nil {
			return err
		}
	default:
		e.ruleRoot = e.root
		if err := e.evalRules(patternRules); err != nil {
			return err
		}
	}

	return nil
}

func (e *Evaluator) GetRootJson() (string, error) {
	val, err := e.root.Value.ToGoValue()
	if err != nil {
		return "", err
	}

	bytes, err := json.MarshalIndent(val, "", "  ")
	if err != nil {
		return "", err
	}
	return string(bytes), nil
}

func EvalExpression(exprSrc string, rootValue interface{}, stdout io.Writer) (*Cell, error) {
	lex := NewLexer(exprSrc)
	parser := NewParser(&lex)
	expr, err := parser.ParseExpression()
	if err != nil {
		return nil, err
	}
	rootCell := NewCell(NewValue(rootValue))
	ev := NewEvaluator(Program{}, &lex, stdout)
	ev.root = rootCell
	ev.ruleRoot = rootCell
	cell, err := ev.evalExpr(expr)
	if err != nil && err != errExit {
		return nil, err
	}
	return cell, nil
}

type InputFile struct {
	Name   string
	Reader io.Reader
}

func EvalProgram(progSrc string, files []InputFile, rootSelector string, stdout io.Writer) (*Evaluator, error) {
	lex := NewLexer(progSrc)
	parser := NewParser(&lex)
	prog, err := parser.Parse()
	if err != nil {
		return nil, err
	}
	ev := NewEvaluator(prog, &lex, stdout)

	// begin rules
	for _, rule := range ev.beginRules {
		ev.ruleRoot = NewCell(NewValue(nil))
		if err := ev.evalStatement(rule.Body); err != nil {
			if err == errExit {
				return &ev, nil
			}
			return &ev, err
		}
	}

	// for each file, run the pattern rules
	for _, file := range files {
		// read the json
		var rootValue interface{}
		b, err := io.ReadAll(file.Reader)
		if err != nil {
			return &ev, err
		}
		err = json.Unmarshal(b, &rootValue)
		if err != nil {
			return &ev, err
		}

		// find the root value
		var rootCell *Cell
		if len(rootSelector) > 0 {
			cell, err := EvalExpression(rootSelector, rootValue, stdout)
			if err != nil {
				return &ev, err
			}
			rootCell = cell
		} else {
			rootCell = NewCell(NewValue(rootValue))
		}

		// run the rules
		ev.setGlobal("$file", NewCell(NewValue(file.Name)))
		ev.root = rootCell
		if err := ev.evalPatternRules(ev.patternRules); err != nil {
			if err == errExit {
				return &ev, nil
			}
			return &ev, err
		}
	}

	// end rules
	for _, rule := range ev.endRules {
		ev.ruleRoot = NewCell(NewValue(nil))
		if err := ev.evalStatement(rule.Body); err != nil {
			if err == errExit {
				return &ev, nil
			}
			return &ev, err
		}
	}

	return &ev, nil
}
