package lang

import (
	"encoding/json"
	"fmt"
	"io"
	"strconv"
	"strings"
)

type Evaluator struct {
	rules    []Rule
	lexer    *Lexer
	stdout   io.Writer
	stdin    io.Reader
	root     *Cell
	ruleRoot *Cell
	locals   map[string]*Cell
}

func NewEvaluator(rules []Rule, lexer *Lexer, stdout io.Writer, stdin io.Reader) Evaluator {
	return Evaluator{
		rules:  rules,
		lexer:  lexer,
		stdout: stdout,
		stdin:  stdin,
		locals: make(map[string]*Cell),
	}
}

func (e *Evaluator) getVariable(name string) (*Cell, error) {
	cell, present := e.locals[name]
	if !present {
		// $fields don't get inferred values
		if strings.HasPrefix(name, "$") {
			return nil, fmt.Errorf("unknown variable %s", name)
		}
		cell = NewCell(Value{Tag: ValueUnknown})
		e.locals[name] = cell
		return cell, nil
	}
	return cell, nil
}

func (e *Evaluator) evalExpr(expr Expr) (*Cell, error) {
	switch exp := expr.(type) {
	case *ExprString:
		str := e.lexer.GetString(&exp.Token)
		return NewCell(Value{
			Tag: ValueStr,
			Str: &str,
		}), nil
	case *ExprNum:
		numStr := e.lexer.GetString(&exp.Token)
		num, err := strconv.ParseInt(numStr, 10, 64)
		if err != nil {
			return nil, err
		}
		f := float64(num)
		return NewCell(Value{
			Tag: ValueNum,
			Num: &f,
		}), nil
	case *ExprBinary:
		return e.evalBinaryExpr(exp)
	case *ExprIdentifier:
		if exp.Token.Tag == Dollar {
			return e.ruleRoot, nil
		} else {
			ident := e.lexer.GetString(&exp.Token)
			local, err := e.getVariable(ident)
			if err != nil {
				return nil, err
			}
			return local, nil
		}
	default:
		return nil, fmt.Errorf("expected an expression but found %T", exp)
	}
}

func (e *Evaluator) evalBinaryExpr(expr *ExprBinary) (*Cell, error) {
	left, err := e.evalExpr(expr.Left)
	if err != nil {
		return nil, err
	}
	right, err := e.evalExpr(expr.Right)
	if err != nil {
		return nil, err
	}

	switch expr.OpToken.Tag {
	case LSquare, Dot:
		member, err := left.Value.GetMember(right.Value)
		if err != nil {
			return nil, err
		}
		return member, nil
	case LessThan, GreaterThan, EqualEqual:
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
			return nil, err
		}
		switch expr.OpToken.Tag {
		case LessThan:
			return NewCell(NewValue(cmp < 0)), nil
		case GreaterThan:
			return NewCell(NewValue(cmp > 0)), nil
		case EqualEqual:
			return NewCell(NewValue(cmp == 0)), nil
		default:
			panic("unhandled comparison operator")
		}
	case Plus, Minus, Multiply, Divide:
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
	case Equal:
		return e.evalAssignment(left, right)
	default:
		return nil, fmt.Errorf("unknown operator %s", expr.OpToken.Tag)
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

func (e *Evaluator) evalStatement(stmt Statement) error {
	switch st := stmt.(type) {
	case *StatementBlock:
		for _, s := range st.Body {
			if err := e.evalStatement(s); err != nil {
				return err
			}
		}
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
			fmt.Fprintf(e.stdout, "%s", cell.Value.PrettyString(false))
		}
		fmt.Fprint(e.stdout, "\n")
	case *StatementExpr:
		_, err := e.evalExpr(st.Expr)
		if err != nil {
			return err
		}
	default:
		return fmt.Errorf("expected a statement but found %T", st)
	}
	return nil
}

func (e *Evaluator) evalRule(rule *Rule) error {
	return e.evalStatement(rule.Body)
}

func (e *Evaluator) evalPatternRules(patternRules []*Rule) error {
	if e.root == nil {
		return nil
	}

	switch e.root.Value.Tag {
	case ValueArray:
		for i, item := range *e.root.Value.Array {
			e.ruleRoot = item
			e.locals["$index"] = NewCell(NewValue(i))
			for _, rule := range patternRules {
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

				if err := e.evalRule(rule); err != nil {
					return err
				}
			}
		}
	default:
		return fmt.Errorf("unhandled root value type %v", e.root.Value.Tag)
	}

	return nil
}

func (e *Evaluator) Eval() error {
	if e.stdin != nil {
		b, err := io.ReadAll(e.stdin)
		if err != nil {
			return err
		}

		var j interface{}
		err = json.Unmarshal(b, &j)
		if err != nil {
			return err
		}
		e.root = NewCell(NewValue(j))
	}

	beginRules := make([]*Rule, 0)
	endRules := make([]*Rule, 0)
	patternRules := make([]*Rule, 0)

	for _, rule := range e.rules {
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
		if err := e.evalRule(rule); err != nil {
			return err
		}
	}

	if err := e.evalPatternRules(patternRules); err != nil {
		return err
	}

	for _, rule := range endRules {
		if err := e.evalRule(rule); err != nil {
			return err
		}
	}

	return nil
}
