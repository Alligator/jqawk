package lang

import (
	"fmt"
	"strings"
)

type Parser struct {
	lexer           *Lexer
	current         *Token
	previous        *Token
	rules           map[TokenTag]parseRule
	didEndStatement bool
}

type parseRule struct {
	prec   Precedence
	prefix func(*Parser) (Expr, error)
	infix  func(*Parser, Expr) (Expr, error)
}

type Precedence uint8

const (
	PrecNone Precedence = iota
	PrecAssign
	PrecLogical
	PrecComparison
	PrecAddition
	PrecMultiplication
	PrecPrefix
	PrecPostfix
	PrecCall
	PrecGroup
)

func NewParser(l *Lexer) Parser {
	p := Parser{
		lexer:           l,
		didEndStatement: false,
	}
	p.rules = map[TokenTag]parseRule{
		Str:           {PrecNone, literal, nil},
		Num:           {PrecNone, literal, nil},
		True:          {PrecNone, literal, nil},
		False:         {PrecNone, literal, nil},
		Dollar:        {PrecNone, identifier, nil},
		Ident:         {PrecNone, identifier, nil},
		LSquare:       {PrecCall, array, computedMember},
		Dot:           {PrecCall, nil, member},
		LParen:        {PrecCall, group, call},
		LessThan:      {PrecComparison, nil, binary},
		GreaterThan:   {PrecComparison, nil, binary},
		EqualEqual:    {PrecComparison, nil, binary},
		BangEqual:     {PrecComparison, nil, binary},
		LessEqual:     {PrecComparison, nil, binary},
		GreaterEqual:  {PrecComparison, nil, binary},
		Tilde:         {PrecComparison, nil, binary},
		BangTilde:     {PrecComparison, nil, binary},
		Equal:         {PrecAssign, nil, binary},
		Plus:          {PrecAddition, unary, binary},
		Minus:         {PrecAddition, unary, binary},
		Multiply:      {PrecMultiplication, nil, binary},
		Divide:        {PrecMultiplication, regex, binary},
		PlusEqual:     {PrecAssign, nil, binary},
		MinusEqual:    {PrecAssign, nil, binary},
		MultiplyEqual: {PrecAssign, nil, binary},
		DivideEqual:   {PrecAssign, nil, binary},
		AmpAmp:        {PrecLogical, nil, binary},
		PipePipe:      {PrecLogical, nil, binary},
		Match:         {PrecNone, match, nil},
		Bang:          {PrecPrefix, unary, nil},
		PlusPlus:      {PrecPostfix, prefix, postfix},
		MinusMinus:    {PrecPostfix, prefix, postfix},
		LCurly:        {PrecNone, object, nil},
		Percent:       {PrecMultiplication, nil, binary},
	}
	return p
}

func (p *Parser) rule(tag TokenTag) parseRule {
	r, present := p.rules[tag]
	if !present {
		return parseRule{PrecNone, nil, nil}
	}
	return r
}

func (p *Parser) atEnd() bool {
	return p.current.Tag == EOF
}

func (p *Parser) error(pos int, msg string) SyntaxError {
	srcLine, line, col := p.lexer.GetLineAndCol(pos)
	return SyntaxError{
		Message: msg,
		Line:    line,
		Col:     col,
		SrcLine: srcLine,
	}
}

func (p *Parser) advance() (Token, error) {
	t, err := p.lexer.Next()
	if err != nil {
		return t, err
	}
	p.previous = p.current
	p.current = &t
	p.didEndStatement = false
	return t, nil
}

func (p *Parser) consume(tags ...TokenTag) error {
	match := false
	for _, tag := range tags {
		if p.current.Tag == tag {
			match = true
			break
		}
	}

	if !match {
		if len(tags) == 1 {
			return p.error(p.current.Pos, fmt.Sprintf("expected %s", tags[0]))
		}

		var sb strings.Builder
		for index, tag := range tags {
			if index > 0 {
				sb.WriteString(", ")
			}
			sb.WriteString(tag.String())
		}
		return p.error(p.current.Pos, fmt.Sprintf("expected one of %s", sb.String()))
	}

	_, err := p.advance()
	return err
}

func (p *Parser) block() (StatementBlock, error) {
	if err := p.consume(LCurly); err != nil {
		return StatementBlock{}, err
	}
	startToken := *p.previous

	block := make([]Statement, 0)
	for !p.atEnd() && p.current.Tag != RCurly {
		statement, err := p.statement()
		if err != nil {
			return StatementBlock{}, err
		}
		block = append(block, statement)
		if !p.atStatementEnd() {
			return StatementBlock{}, p.error(p.current.Pos, "unexpected end of input")
		}
	}
	if err := p.consume(RCurly); err != nil {
		return StatementBlock{}, err
	}
	p.didEndStatement = true
	return StatementBlock{startToken, block}, nil
}

func (p *Parser) statement() (Statement, error) {
	p.didEndStatement = false
	switch p.current.Tag {
	case Print:
		statement, err := p.printStatement()
		if err != nil {
			return nil, err
		}
		return &statement, nil
	case Return:
		if err := p.consume(Return); err != nil {
			return nil, err
		}
		expr, err := p.expression()
		if err != nil {
			return nil, err
		}
		return &StatementReturn{expr}, nil
	case If:
		if err := p.consume(If); err != nil {
			return nil, err
		}
		if err := p.consume(LParen); err != nil {
			return nil, err
		}

		expr, err := p.expression()
		if err != nil {
			return nil, err
		}

		if err := p.consume(RParen); err != nil {
			return nil, err
		}

		body, err := p.statement()
		if err != nil {
			return nil, err
		}

		var elseBody Statement
		if p.current.Tag == Else {
			if err := p.consume(Else); err != nil {
				return nil, err
			}
			elseBody, err = p.statement()
			if err != nil {
				return nil, err
			}
		}

		return &StatementIf{expr, body, elseBody}, nil
	case While:
		if err := p.consume(While); err != nil {
			return nil, err
		}
		if err := p.consume(LParen); err != nil {
			return nil, err
		}

		expr, err := p.expression()
		if err != nil {
			return nil, err
		}

		if err := p.consume(RParen); err != nil {
			return nil, err
		}

		body, err := p.statement()
		if err != nil {
			return nil, err
		}

		return &StatementWhile{expr, body}, nil
	case For:
		// for (
		if err := p.consume(For); err != nil {
			return nil, err
		}
		if err := p.consume(LParen); err != nil {
			return nil, err
		}

		preExpr, err := p.expression()
		if err != nil {
			return nil, err
		}

		// try to parse for (ident[, ident] in expr)
		if ident, ok := preExpr.(*ExprIdentifier); ok {
			if p.current.Tag == In || p.current.Tag == Comma {
				var indexIdent *ExprIdentifier
				if p.current.Tag == Comma {
					p.consume(Comma)
					if err := p.consume(Ident); err != nil {
						return nil, err
					}
					indexIdent = &ExprIdentifier{*p.previous}
				}

				p.consume(In)
				expr, err := p.expression()
				if err != nil {
					return nil, err
				}

				if err := p.consume(RParen); err != nil {
					return nil, err
				}

				body, err := p.statement()
				if err != nil {
					return nil, err
				}

				return &StatementForIn{ident, indexIdent, expr, body}, nil
			}
		}

		// try to parse for (expr; expr; expr)
		if err := p.consume(SemiColon); err != nil {
			return nil, err
		}

		expr, err := p.expression()
		if err != nil {
			return nil, err
		}
		if err := p.consume(SemiColon); err != nil {
			return nil, err
		}

		postExpr, err := p.expression()
		if err != nil {
			return nil, err
		}

		if err := p.consume(RParen); err != nil {
			return nil, err
		}

		body, err := p.statement()
		if err != nil {
			return nil, err
		}

		return &StatementFor{preExpr, expr, postExpr, body}, nil
	case LCurly:
		block, err := p.block()
		if err != nil {
			return nil, err
		}
		return &block, nil
	case Break:
		p.consume(Break)
		stmt := StatementBreak{*p.previous}
		return &stmt, nil
	case Continue:
		p.consume(Continue)
		stmt := StatementContinue{*p.previous}
		return &stmt, nil
	case Next:
		p.consume(Next)
		stmt := StatementNext{*p.previous}
		return &stmt, nil
	default:
		expr, err := p.expression()
		if err != nil {
			return nil, err
		}
		return &StatementExpr{expr}, nil
	}
}

func (p *Parser) printStatement() (StatementPrint, error) {
	if err := p.consume(Print); err != nil {
		return StatementPrint{}, err
	}

	startToken := *p.previous

	args := make([]Expr, 0)
	for !p.atStatementEnd() {
		expr, err := p.expression()
		if err != nil {
			return StatementPrint{}, err
		}
		args = append(args, expr)
		if p.current.Tag == Comma {
			p.consume(Comma)
		} else {
			break
		}
	}

	if p.atStatementEnd() {
		p.didEndStatement = true
	}
	return StatementPrint{startToken, args}, nil
}

func (p *Parser) atStatementEnd() bool {
	if p.didEndStatement {
		return true
	}

	switch p.current.Tag {
	case RCurly:
		return true
	case SemiColon:
		p.consume(SemiColon)
		return true
	default:
		return false
	}
}

func (p *Parser) expression() (Expr, error) {
	return p.expressionWithPrec(PrecAssign)
}

func (p *Parser) expressionWithPrec(prec Precedence) (Expr, error) {
	prefixRule := p.rule(p.current.Tag)
	if prefixRule.prefix == nil {
		return nil, p.error(p.current.Pos, fmt.Sprintf("unexpected token %s", p.current.Tag))
	}

	lhs, err := prefixRule.prefix(p)
	if err != nil {
		return nil, err
	}

	for prec <= p.rule(p.current.Tag).prec {
		infixRule := p.rule(p.current.Tag)
		if infixRule.infix == nil {
			return nil, p.error(p.current.Pos, fmt.Sprintf("unknown operator %s", p.current.Tag))
		}
		lhs, err = infixRule.infix(p, lhs)
		if err != nil {
			return nil, err
		}
	}

	return lhs, nil
}

func literal(p *Parser) (Expr, error) {
	if _, err := p.advance(); err != nil {
		return nil, err
	}
	return &ExprLiteral{*p.previous}, nil
}

func regex(p *Parser) (Expr, error) {
	token, err := p.lexer.regex()
	if err != nil {
		return nil, err
	}
	if token.Tag != Regex {
		panic(fmt.Errorf("expected a regex token but got %s", token.Tag))
	}
	p.current = &token
	if _, err := p.advance(); err != nil {
		return nil, err
	}
	return &ExprLiteral{token}, nil
}

func identifier(p *Parser) (Expr, error) {
	switch p.current.Tag {
	case Dollar, Ident:
		if _, err := p.advance(); err != nil {
			return nil, err
		}
		return &ExprIdentifier{*p.previous}, nil
	}
	return nil, p.error(p.current.Pos, "expected an identifier")
}

func array(p *Parser) (Expr, error) {
	if err := p.consume(LSquare); err != nil {
		return nil, err
	}
	token := p.previous

	items, err := p.evalExprList(RSquare)
	if err != nil {
		return nil, err
	}

	return &ExprArray{*token, items}, nil
}

func object(p *Parser) (Expr, error) {
	if err := p.consume(LCurly); err != nil {
		return nil, err
	}
	token := *p.previous

	items := make([]ObjectKeyValue, 0)
	for p.current.Tag != RCurly && !p.atEnd() {
		err := p.consume(Str, Ident)
		if err != nil {
			return nil, err
		}
		key := p.lexer.GetString(p.previous)
		if err = p.consume(Colon); err != nil {
			return nil, err
		}
		value, err := p.expression()
		if err != nil {
			return nil, err
		}
		items = append(items, ObjectKeyValue{key, value})

		if p.current.Tag == Comma {
			if err = p.consume(Comma); err != nil {
				return nil, err
			}
		}
	}

	if err := p.consume(RCurly); err != nil {
		return nil, err
	}

	return &ExprObject{token, items}, nil
}

func computedMember(p *Parser, left Expr) (Expr, error) {
	opToken := p.current
	if err := p.consume(LSquare); err != nil {
		return nil, err
	}

	expr, err := p.expression()
	if err != nil {
		return nil, err
	}

	if err := p.consume(RSquare); err != nil {
		return nil, err
	}

	return &ExprBinary{
		Left:    left,
		Right:   expr,
		OpToken: *opToken,
	}, nil
}

func member(p *Parser, left Expr) (Expr, error) {
	if err := p.consume(Dot); err != nil {
		return nil, err
	}
	opToken := p.previous

	if err := p.consume(Ident); err != nil {
		return nil, err
	}
	ident := p.previous

	return &ExprBinary{
		Left:    left,
		Right:   &ExprLiteral{*ident},
		OpToken: *opToken,
	}, nil
}

func (p *Parser) evalExprList(endToken TokenTag) ([]Expr, error) {
	args := make([]Expr, 0)
	for !p.atEnd() && p.current.Tag != endToken {
		expr, err := p.expression()
		if err != nil {
			return nil, err
		}
		args = append(args, expr)
		if p.current.Tag == Comma {
			p.consume(Comma)
		} else {
			break
		}
	}
	if err := p.consume(endToken); err != nil {
		return nil, err
	}
	return args, nil
}

func call(p *Parser, left Expr) (Expr, error) {
	if err := p.consume(LParen); err != nil {
		return nil, err
	}

	args, err := p.evalExprList(RParen)
	if err != nil {
		return nil, err
	}

	return &ExprCall{
		Func: left,
		Args: args,
	}, nil
}

func match(p *Parser) (Expr, error) {
	if err := p.consume(Match); err != nil {
		return nil, err
	}
	token := *p.previous

	if err := p.consume(LParen); err != nil {
		return nil, err
	}
	expr, err := p.expression()
	if err != nil {
		return nil, err
	}
	if err := p.consume(RParen); err != nil {
		return nil, err
	}

	if err := p.consume(LCurly); err != nil {
		return nil, err
	}

	match := ExprMatch{
		token: token,
		Value: expr,
		Cases: make([]MatchCase, 0),
	}

	for p.current.Tag != RCurly && !p.atEnd() {
		exprs := make([]Expr, 0)
		for !p.atEnd() {
			caseExpr, err := p.expression()
			if err != nil {
				return nil, err
			}
			exprs = append(exprs, caseExpr)
			if p.current.Tag != Comma {
				break
			}
			p.consume(Comma)
		}

		if err := p.consume(Arrow); err != nil {
			return nil, err
		}

		// a case can either be a block, or an expression
		var caseBody Statement
		if p.current.Tag == LCurly {
			// if it looks like a block, parse a statement
			caseBody, err = p.statement()
			if err != nil {
				return nil, err
			}
		} else {
			// otherwise parse an expr, and stuff if into a StatementExpr
			expr, err := p.expression()
			if err != nil {
				return nil, err
			}
			caseBody = &StatementExpr{
				Expr: expr,
			}
		}

		matchCase := MatchCase{
			Exprs: exprs,
			Body:  caseBody,
		}

		match.Cases = append(match.Cases, matchCase)

		if p.current.Tag == Comma {
			if _, err := p.advance(); err != nil {
				return nil, err
			}
		}
	}

	if err := p.consume(RCurly); err != nil {
		return nil, err
	}
	p.didEndStatement = true

	return &match, nil
}

func unary(p *Parser) (Expr, error) {
	_, err := p.advance()
	if err != nil {
		return nil, err
	}
	opToken := *p.previous

	expr, err := p.expression()
	if err != nil {
		return nil, err
	}

	return &ExprUnary{
		Expr:    expr,
		OpToken: opToken,
	}, nil
}

func group(p *Parser) (Expr, error) {
	if err := p.consume(LParen); err != nil {
		return nil, err
	}

	expr, err := p.expression()
	if err != nil {
		return nil, err
	}

	if err := p.consume(RParen); err != nil {
		return nil, err
	}

	return expr, nil
}

func prefix(p *Parser) (Expr, error) {
	_, err := p.advance()
	if err != nil {
		return nil, err
	}
	opToken := *p.previous

	expr, err := p.expressionWithPrec(p.rule(opToken.Tag).prec)
	if err != nil {
		return nil, err
	}

	return &ExprUnary{
		Expr:    expr,
		OpToken: opToken,
		Postfix: false,
	}, nil
}

func postfix(p *Parser, left Expr) (Expr, error) {
	_, err := p.advance()
	if err != nil {
		return nil, err
	}
	opToken := *p.previous

	return &ExprUnary{
		Expr:    left,
		OpToken: opToken,
		Postfix: true,
	}, nil
}

func binary(p *Parser, left Expr) (Expr, error) {
	_, err := p.advance()
	if err != nil {
		return nil, err
	}
	opToken := *p.previous

	expr, err := p.expressionWithPrec(p.rule(opToken.Tag).prec)
	if err != nil {
		return nil, err
	}

	switch opToken.Tag {
	case PlusEqual, MinusEqual, MultiplyEqual, DivideEqual:
		return p.rewriteCompundAssingment(left, expr, opToken)
	default:
		return &ExprBinary{
			Left:    left,
			Right:   expr,
			OpToken: opToken,
		}, nil
	}
}

func (p *Parser) rewriteCompundAssingment(left Expr, right Expr, opToken Token) (Expr, error) {
	// a += b -> a = a + b
	var opTag TokenTag
	switch opToken.Tag {
	case PlusEqual:
		opTag = Plus
	case MinusEqual:
		opTag = Minus
	case MultiplyEqual:
		opTag = Multiply
	case DivideEqual:
		opTag = Divide
	default:
		panic(fmt.Errorf("attempted compound assignment with %s", opToken.Tag))
	}

	equalOp := Token{
		Tag: Equal,
		Pos: opToken.Pos,
	}
	op := Token{
		Tag: opTag,
		Pos: opToken.Pos,
		Len: opToken.Len,
	}

	return &ExprBinary{
		Left: left,
		Right: &ExprBinary{
			Left:    left,
			Right:   right,
			OpToken: op,
		},
		OpToken: equalOp,
	}, nil
}

func (p *Parser) parseRule() (Rule, error) {
	rule := Rule{}
	switch p.current.Tag {
	case Begin:
		rule.Kind = BeginRule
		if err := p.consume(Begin); err != nil {
			return rule, err
		}
	case End:
		rule.Kind = EndRule
		if err := p.consume(End); err != nil {
			return rule, err
		}
	case LCurly:
		rule.Kind = PatternRule
	default:
		rule.Kind = PatternRule
		pat, err := p.expression()
		if err != nil {
			return rule, err
		}
		rule.Pattern = pat
	}

	if p.current.Tag == LCurly {
		body, err := p.block()
		if err != nil {
			return rule, err
		}
		rule.Body = &body
	} else {
		// rule with no body
		// becomes { print }
		rule.Body = &StatementPrint{}
	}

	return rule, nil
}

func (p *Parser) parseFunction() (ExprFunction, error) {
	if err := p.consume(Function); err != nil {
		return ExprFunction{}, err
	}

	if err := p.consume(Ident); err != nil {
		return ExprFunction{}, err
	}

	identToken := *p.previous

	if err := p.consume(LParen); err != nil {
		return ExprFunction{}, err
	}

	args := make([]string, 0)
	for !p.atEnd() && p.current.Tag != RParen {
		if err := p.consume(Ident); err != nil {
			return ExprFunction{}, err
		}
		str := p.lexer.GetString(p.previous)
		args = append(args, str)
		if p.current.Tag == Comma {
			p.consume(Comma)
		}
	}

	if err := p.consume(RParen); err != nil {
		return ExprFunction{}, err
	}

	block, err := p.block()
	if err != nil {
		return ExprFunction{}, err
	}

	return ExprFunction{
		ident: identToken,
		Args:  args,
		Body:  &block,
	}, nil
}

func (p *Parser) ParseExpression() (Expr, error) {
	if _, err := p.advance(); err != nil {
		return nil, err
	}
	expr, err := p.expression()
	if err != nil {
		return nil, err
	}
	if err = p.consume(EOF); err != nil {
		return nil, err
	}
	return expr, nil
}

func (p *Parser) Parse() (Program, error) {
	prog := Program{}
	rules := make([]Rule, 0)
	functions := make([]ExprFunction, 0)
	if _, err := p.advance(); err != nil {
		return prog, err
	}
	for !p.atEnd() {
		if p.current.Tag == Function {
			fn, err := p.parseFunction()
			if err != nil {
				return prog, err
			}
			functions = append(functions, fn)
			continue
		}
		rule, err := p.parseRule()
		if err != nil {
			return prog, err
		}
		rules = append(rules, rule)
	}
	prog.Rules = rules
	prog.Functions = functions
	return prog, nil
}
