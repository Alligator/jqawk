package lang

type (
	Node interface {
		Token() Token
	}
	Expr interface {
		Node
		exprNode()
	}
	Statement interface {
		Node
		statementNode()
	}
)

type RuleKind uint8

const (
	BeginRule RuleKind = iota
	EndRule
	PatternRule
)

func (k RuleKind) String() string {
	switch k {
	case BeginRule:
		return "BeginRule"
	case EndRule:
		return "EndRule"
	case PatternRule:
		return "PatternRule"
	default:
		return "???"
	}
}

type Program struct {
	Rules     []Rule
	Functions []ExprFunction
}

type Rule struct {
	Kind    RuleKind
	Pattern Expr
	Body    Statement
}

type ExprLiteral struct {
	token Token
}

type ExprIdentifier struct {
	token Token
}

type ExprArray struct {
	token Token
	Items []Expr
}

type ExprObject struct {
	token Token
	Items []ObjectKeyValue
}

type ObjectKeyValue struct {
	Key   string
	Value Expr
}

type ExprUnary struct {
	Expr    Expr
	OpToken Token
	Postfix bool
}

type ExprBinary struct {
	Left    Expr
	Right   Expr
	OpToken Token
}

type ExprCall struct {
	Func Expr
	Args []Expr
}

type ExprFunction struct {
	ident Token
	Args  []string
	Body  Statement
}

type ExprMatch struct {
	token Token
	Value Expr
	Cases []MatchCase
}

type MatchCase struct {
	Exprs []Expr
	Body  Statement
}

func (*ExprLiteral) exprNode()    {}
func (*ExprIdentifier) exprNode() {}
func (*ExprArray) exprNode()      {}
func (*ExprObject) exprNode()     {}
func (*ExprUnary) exprNode()      {}
func (*ExprBinary) exprNode()     {}
func (*ExprCall) exprNode()       {}
func (*ExprFunction) exprNode()   {}
func (*ExprMatch) exprNode()      {}

func (expr *ExprLiteral) Token() Token    { return expr.token }
func (expr *ExprIdentifier) Token() Token { return expr.token }
func (expr *ExprArray) Token() Token      { return expr.token }
func (expr *ExprObject) Token() Token     { return expr.token }
func (expr *ExprUnary) Token() Token      { return expr.OpToken }
func (expr *ExprBinary) Token() Token     { return expr.Left.Token() }
func (expr *ExprCall) Token() Token       { return expr.Func.Token() }
func (expr *ExprFunction) Token() Token   { return expr.ident }
func (expr *ExprMatch) Token() Token      { return expr.token }

type StatementBlock struct {
	token Token
	Body  []Statement
}

type StatementPrint struct {
	token Token
	Args  []Expr
}

type StatementExpr struct {
	Expr Expr
}

type StatementReturn struct {
	Expr Expr
}

type StatementBreak struct {
	token Token
}

type StatementIf struct {
	Expr     Expr
	Body     Statement
	ElseBody Statement
}

type StatementWhile struct {
	Expr Expr
	Body Statement
}

type StatementFor struct {
	PreExpr  Expr
	Expr     Expr
	PostExpr Expr
	Body     Statement
}

type StatementForIn struct {
	Ident    *ExprIdentifier
	Iterable Expr
	Body     Statement
}

func (*StatementBlock) statementNode()  {}
func (*StatementPrint) statementNode()  {}
func (*StatementExpr) statementNode()   {}
func (*StatementReturn) statementNode() {}
func (*StatementBreak) statementNode()  {}
func (*StatementIf) statementNode()     {}
func (*StatementWhile) statementNode()  {}
func (*StatementFor) statementNode()    {}
func (*StatementForIn) statementNode()  {}

func (stmt *StatementBlock) Token() Token  { return stmt.token }
func (stmt *StatementPrint) Token() Token  { return stmt.token }
func (stmt *StatementExpr) Token() Token   { return stmt.Expr.Token() }
func (stmt *StatementReturn) Token() Token { return stmt.Expr.Token() }
func (stmt *StatementBreak) Token() Token  { return stmt.token }
func (stmt *StatementIf) Token() Token     { return stmt.Expr.Token() }
func (stmt *StatementWhile) Token() Token  { return stmt.Expr.Token() }
func (stmt *StatementFor) Token() Token    { return stmt.Expr.Token() }
func (stmt *StatementForIn) Token() Token  { return stmt.Ident.Token() }
