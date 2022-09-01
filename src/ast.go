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

func (*ExprString) exprNode()     {}
func (*ExprNum) exprNode()        {}
func (*ExprIdentifier) exprNode() {}
func (*ExprBinary) exprNode()     {}

type StatementBlock struct {
	Body []Statement
}

type StatementPrint struct {
	Args []Expr
}

func (*StatementBlock) statementNode() {}
func (*StatementPrint) statementNode() {}
