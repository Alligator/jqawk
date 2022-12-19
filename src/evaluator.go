package lang

import (
	"encoding/json"
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
	prog     Program
	lexer    *Lexer
	stdout   io.Writer
	root     *Cell
	ruleRoot *Cell
	stackTop *stackFrame
}

type statementAction uint8

const (
	StmtActionNone statementAction = iota
	StmtActionReturn
)

func NewEvaluator(prog Program, lexer *Lexer, stdout io.Writer) Evaluator {
	e := Evaluator{
		prog:   prog,
		lexer:  lexer,
		stdout: stdout,
	}
	e.pushFrame("<root>")
	addRuntimeFunctions(&e)
	e.addProgramFunctions()
	return e
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
		case '\t':
			buf = append(buf, '\t')
		default:
			return nil, fmt.Errorf("unknown escape char %q", str[i])
		}
	}
	s := string(buf)
	return NewCell(Value{
		Tag: ValueStr,
		Str: &s,
	}), nil
}

func (e *Evaluator) evalExpr(expr Expr) (*Cell, error) {
	switch exp := expr.(type) {
	case *ExprString:
		str := e.lexer.GetString(&exp.token)
		return e.evalString(str)
	case *ExprRegex:
		str := e.lexer.GetString(&exp.token)
		val := Value{
			Tag: ValueRegex,
			Str: &str,
		}
		return NewCell(val), nil
	case *ExprNum:
		numStr := e.lexer.GetString(&exp.token)
		num, err := strconv.ParseInt(numStr, 10, 64)
		if err != nil {
			return nil, err
		}
		f := float64(num)
		return NewCell(Value{
			Tag: ValueNum,
			Num: &f,
		}), nil
	case *ExprUnary:
		return e.evalUnaryExpr(exp)
	case *ExprBinary:
		return e.evalBinaryExpr(exp)
	case *ExprIdentifier:
		if exp.token.Tag == Dollar {
			if e.ruleRoot != nil {
				return e.ruleRoot, nil
			} else {
				return nil, e.error(exp.Token(), "unknown variable $")
			}
		} else {
			ident := e.lexer.GetString(&exp.token)
			local, err := e.getVariable(ident)
			if err != nil {
				return nil, e.error(exp.Token(), err.Error())
			}
			return local, nil
		}
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

		result, err := e.callFunction(fn, argVals)
		if err != nil {
			return nil, e.error(exp.Token(), err.Error())
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
			isMatch, err := e.evalCaseMatch(value, matchCase.Expr)
			if err != nil {
				return nil, err
			}
			if isMatch {
				e.pushFrame("<match>")

				switch exp := matchCase.Expr.(type) {
				case *ExprIdentifier:
					ident := e.lexer.GetString(&exp.token)
					e.stackTop.locals[ident] = value
				}

				// TODO deal with StatementAction
				_, value, err := e.evalStatement(matchCase.Body)
				if err != nil {
					return nil, err
				}

				if err := e.popFrame(); err != nil {
					return nil, err
				}
				return value, nil
			}
		}
		return NewCell(NewValue(nil)), nil
	default:
		return nil, e.error(exp.Token(), "expected an expression")
	}
}

func (e *Evaluator) evalCaseMatch(value *Cell, expr Expr) (bool, error) {
	switch expr.(type) {
	case *ExprIdentifier:
		return true, nil
	case *ExprNum, *ExprString:
		caseValue, err := e.evalExpr(expr)
		if err != nil {
			return false, err
		}
		cmp, err := value.Value.Compare(&caseValue.Value)
		if err != nil {
			return false, err
		}
		return cmp == 0, nil
	}
	return false, e.error(expr.Token(), "only numbers and strings are supported in match expressions")
}

func (e *Evaluator) callFunction(fn *Cell, args []*Value) (*Cell, error) {
	switch fn.Value.Tag {
	case ValueNativeFn:
		result, err := fn.Value.NativeFn(e, args, fn.Value.Binding)
		if err != nil {
			return nil, err
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

		action, value, err := e.evalStatement(f.Body)
		if err != nil {
			return nil, err
		}

		var retCell *Cell
		if action == StmtActionReturn {
			retCell = value
		} else {
			retCell = NewCell(NewValue(nil))
		}

		if err := e.popFrame(); err != nil {
			return nil, err
		}

		return retCell, nil
	default:
		return nil, fmt.Errorf("attempted to call a %s", fn.Value.Tag)
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
			// if it's unknown, make it an object
			left.Value = NewObject()
		}
		member, err := left.Value.GetMember(right.Value)
		if err != nil {
			return nil, e.error(expr.Left.Token(), err.Error())
		}
		if member == nil {
			// auto-create members
			member, err = left.Value.SetMember(right.Value, NewCell(NewValue(nil)))
			if err != nil {
				return nil, e.error(expr.Left.Token(), err.Error())
			}
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
	case Plus, Minus, Multiply, Divide:
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
			return NewCell(NewValue(leftNum / rightNum)), nil
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
			return nil, err
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
		return e.evalAssignment(left, right)
	default:
		return nil, e.error(expr.OpToken, fmt.Sprintf("unknown operator %s", expr.OpToken.Tag))
	}
}

func (e *Evaluator) evalAssignment(left *Cell, right *Cell) (*Cell, error) {
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
		left.Value = Value{
			Tag: ValueStr,
			Str: &s,
		}

	// reference
	case ValueArray, ValueObj:
		left.Value = right.Value

	default:
		panic(fmt.Errorf("assignment not implemented for %s", right.Value.Tag))
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

func (e *Evaluator) evalStatement(stmt Statement) (statementAction, *Cell, error) {
	switch st := stmt.(type) {
	case *StatementBlock:
		var lastValue *Cell
		for _, s := range st.Body {
			action, value, err := e.evalStatement(s)
			lastValue = value
			if err != nil {
				return 0, nil, err
			}
			if action == StmtActionReturn {
				return action, value, nil
			}
			lastValue = value
		}
		return StmtActionNone, lastValue, nil
	case *StatementPrint:
		args, err := e.evalExprList(st.Args)
		if err != nil {
			return 0, nil, err
		}

		if len(args) == 0 {
			fmt.Fprintln(e.stdout, e.ruleRoot.Value.PrettyString(false))
			return StmtActionNone, nil, nil
		}

		for i, cell := range args {
			if i > 0 {
				fmt.Fprint(e.stdout, " ")
			}
			if cell == nil {
				fmt.Fprint(e.stdout, "nil")
			} else {
				fmt.Fprintf(e.stdout, "%s", cell.Value.PrettyString(false))
			}
		}
		fmt.Fprint(e.stdout, "\n")
	case *StatementExpr:
		value, err := e.evalExpr(st.Expr)
		if err != nil {
			return 0, nil, err
		}
		return 0, value, nil
	case *StatementReturn:
		cell, err := e.evalExpr(st.Expr)
		if err != nil {
			return StmtActionReturn, nil, err
		}
		return StmtActionReturn, cell, nil
	case *StatementIf:
		cell, err := e.evalExpr(st.Expr)
		if err != nil {
			return 0, nil, err
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
				return 0, nil, err
			}
			if cell.Value.isTruthy() {
				action, _, err := e.evalStatement(st.Body)
				if err != nil {
					return 0, nil, err
				}
				if action == StmtActionReturn {
					return action, nil, nil
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
				return 0, nil, err
			}
			if cell.Value.isTruthy() {
				action, _, err := e.evalStatement(st.Body)
				if err != nil {
					return 0, nil, err
				}
				if action == StmtActionReturn {
					return action, nil, nil
				}

				_, err = e.evalExpr(st.PostExpr)
				if err != nil {
					return 0, nil, err
				}
			} else {
				break
			}
		}
	case *StatementForIn:
		ident := e.lexer.GetString(&st.Ident.token)
		local, err := e.getVariable(ident)
		if err != nil {
			return 0, nil, err
		}

		iterable, err := e.evalExpr(st.Iterable)
		if err != nil {
			return 0, nil, err
		}

		switch iterable.Value.Tag {
		case ValueArray:
			for _, item := range iterable.Value.Array {
				local.Value = item.Value
				e.evalStatement(st.Body)
			}
		default:
			return 0, nil, e.error(st.Iterable.Token(), fmt.Sprintf("%s is not iterable", iterable.Value.Tag))
		}

	default:
		return 0, nil, e.error(st.Token(), fmt.Sprintf("expected a statement but found %T", st))
	}
	return StmtActionNone, nil, nil
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

		_, _, err := e.evalStatement(rule.Body)
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
		for key, value := range *e.root.Value.Obj {
			e.ruleRoot = value
			e.stackTop.locals["$key"] = NewCell(NewValue(key))
			if err := e.evalRules(patternRules); err != nil {
				return err
			}
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
	if err != nil {
		return nil, err
	}
	return cell, nil
}

func EvalProgram(progSrc string, rootCell *Cell, stdout io.Writer) (*Evaluator, error) {
	lex := NewLexer(progSrc)
	parser := NewParser(&lex)
	prog, err := parser.Parse()
	if err != nil {
		return nil, err
	}
	ev := NewEvaluator(prog, &lex, stdout)
	err = ev.Eval(rootCell)
	if err != nil {
		return nil, err
	}
	return &ev, nil
}

func (e *Evaluator) Eval(rootCell *Cell) error {
	e.root = rootCell

	beginRules := make([]*Rule, 0)
	endRules := make([]*Rule, 0)
	patternRules := make([]*Rule, 0)

	for _, rule := range e.prog.Rules {
		r := rule
		switch rule.Kind {
		case BeginRule:
			beginRules = append(beginRules, &r)
		case EndRule:
			endRules = append(endRules, &r)
		case PatternRule:
			patternRules = append(patternRules, &r)
		}
	}

	for _, rule := range beginRules {
		if _, _, err := e.evalStatement(rule.Body); err != nil {
			return err
		}
	}

	if err := e.evalPatternRules(patternRules); err != nil {
		return err
	}

	for _, rule := range endRules {
		if _, _, err := e.evalStatement(rule.Body); err != nil {
			return err
		}
	}

	return nil
}
