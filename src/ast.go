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

type ExprString struct {
	token Token
}

type ExprRegex struct {
	token Token
}

type ExprNum struct {
	token Token
}

type ExprIdentifier struct {
	token Token
}

type ExprArray struct {
	token Token
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

func (*ExprString) exprNode()     {}
func (*ExprRegex) exprNode()      {}
func (*ExprNum) exprNode()        {}
func (*ExprIdentifier) exprNode() {}
func (*ExprArray) exprNode()      {}
func (*ExprBinary) exprNode()     {}
func (*ExprCall) exprNode()       {}
func (*ExprFunction) exprNode()   {}

func (expr *ExprString) Token() Token     { return expr.token }
func (expr *ExprRegex) Token() Token      { return expr.token }
func (expr *ExprNum) Token() Token        { return expr.token }
func (expr *ExprIdentifier) Token() Token { return expr.token }
func (expr *ExprArray) Token() Token      { return expr.token }
func (expr *ExprBinary) Token() Token     { return expr.Left.Token() }
func (expr *ExprCall) Token() Token       { return expr.Func.Token() }
func (expr *ExprFunction) Token() Token   { return expr.ident }

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
func (*StatementIf) statementNode()     {}
func (*StatementWhile) statementNode()  {}
func (*StatementFor) statementNode()    {}
func (*StatementForIn) statementNode()  {}

func (stmt *StatementBlock) Token() Token  { return stmt.token }
func (stmt *StatementPrint) Token() Token  { return stmt.token }
func (stmt *StatementExpr) Token() Token   { return stmt.Expr.Token() }
func (stmt *StatementReturn) Token() Token { return stmt.Expr.Token() }
func (stmt *StatementIf) Token() Token     { return stmt.Expr.Token() }
func (stmt *StatementWhile) Token() Token  { return stmt.Expr.Token() }
func (stmt *StatementFor) Token() Token    { return stmt.Expr.Token() }
func (stmt *StatementForIn) Token() Token  { return stmt.Ident.Token() }
