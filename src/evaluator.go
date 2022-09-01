package lang

import (
	"encoding/json"
	"fmt"
	"io"
	"strconv"
)

type Evaluator struct {
	rules    []Rule
	lexer    *Lexer
	stdout   io.Writer
	stdin    io.Reader
	root     *Value
	ruleRoot *Value
	locals   map[string]Value
}

func NewEvaluator(rules []Rule, lexer *Lexer, stdout io.Writer, stdin io.Reader) Evaluator {
	return Evaluator{
		rules:  rules,
		lexer:  lexer,
		stdout: stdout,
		stdin:  stdin,
		locals: make(map[string]Value),
	}
}

func (e *Evaluator) evalExpr(expr Expr) (Value, error) {
	switch exp := expr.(type) {
	case *ExprString:
		str := e.lexer.GetString(&exp.Token)
		return Value{
			Tag: ValueStr,
			Str: &str,
		}, nil
	case *ExprNum:
		numStr := e.lexer.GetString(&exp.Token)
		num, err := strconv.ParseInt(numStr, 10, 64)
		if err != nil {
			return Value{}, err
		}
		f := float64(num)
		return Value{
			Tag: ValueNum,
			Num: &f,
		}, nil
	case *ExprBinary:
		return e.evalBinaryExpr(exp)
	case *ExprIdentifier:
		if exp.Token.Tag == Dollar {
			return *e.ruleRoot, nil
		} else {
			ident := e.lexer.GetString(&exp.Token)
			local, present := e.locals[ident]
			if !present {
				return Value{}, fmt.Errorf("unknown variable %s", ident)
			}
			return local, nil
		}
	default:
		return Value{}, fmt.Errorf("expected an expression but found %T", exp)
	}
}

func (e *Evaluator) evalBinaryExpr(expr *ExprBinary) (Value, error) {
	left, err := e.evalExpr(expr.Left)
	if err != nil {
		return Value{}, err
	}
	right, err := e.evalExpr(expr.Right)
	if err != nil {
		return Value{}, err
	}

	switch expr.OpToken.Tag {
	case LSquare:
		member, err := left.GetMember(right)
		if err != nil {
			return Value{}, err
		}
		return member, nil
	default:
		return Value{}, fmt.Errorf("unknown operator %s", expr.OpToken.Tag)
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
			fmt.Fprintln(e.stdout, e.ruleRoot.String())
			return nil
		}

		for i, arg := range args {
			if i > 0 {
				fmt.Fprint(e.stdout, " ")
			}
			fmt.Fprintf(e.stdout, "%s", arg.String())
		}
		fmt.Fprint(e.stdout, "\n")
	default:
		return fmt.Errorf("expected a statement but found %v", st)
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

	switch e.root.Tag {
	case ValueArray:
		for i, item := range *e.root.Array {
			e.ruleRoot = &item
			e.locals["$index"] = NewValue(i)
			for _, rule := range patternRules {
				if err := e.evalRule(rule); err != nil {
					return err
				}
			}
		}
	default:
		return fmt.Errorf("unhandled root value type %v", e.root.Tag)
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
		v := NewValue(j)
		e.root = &v
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
