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

type Rule struct {
	Kind    RuleKind
	Pattern Expr
	Body    Statement
}

type ExprString struct {
	Token Token
}

type ExprNum struct {
	Token Token
}

type ExprIdentifier struct {
	Token Token
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

func (*ExprString) exprNode()     {}
func (*ExprNum) exprNode()        {}
func (*ExprIdentifier) exprNode() {}
func (*ExprBinary) exprNode()     {}
func (*ExprCall) exprNode()       {}

type StatementBlock struct {
	Body []Statement
}

type StatementPrint struct {
	Args []Expr
}

type StatementExpr struct {
	Expr Expr
}

func (*StatementBlock) statementNode() {}
func (*StatementPrint) statementNode() {}
func (*StatementExpr) statementNode()  {}
