package lang

type (
	Node interface {
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
func (*ExprBinary) exprNode()     {}
func (*ExprCall) exprNode()       {}
func (*ExprFunction) exprNode()   {}

type StatementBlock struct {
	Body []Statement
}

type StatementPrint struct {
	Args []Expr
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

func (*StatementBlock) statementNode()  {}
func (*StatementPrint) statementNode()  {}
func (*StatementExpr) statementNode()   {}
func (*StatementReturn) statementNode() {}
func (*StatementIf) statementNode()     {}
func (*StatementWhile) statementNode()  {}
