use crate::vm::{OpCode, Value};
use crate::lexer::{Lexer, Token, TokenKind};

pub struct Compiler {
  current: Token,
  prev: Token,
  lexer: Lexer,
  output: Vec<OpCode>,
}

#[derive(Clone, PartialEq, Debug)]
pub enum JqaRuleKind {
  Begin,
  Match,
  End,
}

#[derive(Clone, Debug)]
pub struct JqaRule {
  pub pattern: Vec<OpCode>,
  pub body: Vec<OpCode>,
  pub kind: JqaRuleKind,
}

#[derive(PartialOrd, PartialEq)]
enum Precedence {
  None = 0,
  Assignment,
  Equal,
  Comparison,
  Addition,
  Multiplication,
  Func,
}

struct ParseRule {
  prec: Precedence,
  infix: Option<fn(&mut Compiler)>,
  prefix: Option<fn(&mut Compiler)>,
}


impl Compiler {
  pub fn new(lexer: Lexer) -> Compiler {
    Compiler {
      current: Token::new(TokenKind::EOF, 0),
      prev: Token::new(TokenKind::EOF, 0),
      lexer: lexer,
      output: Vec::new(),
    }
  }

  fn get_rule(&mut self, kind: TokenKind) -> ParseRule {
    match kind {
      TokenKind::Dollar => ParseRule {
        prec: Precedence::None,
        prefix: Some(|comp: &mut Compiler| { comp.field() }),
        infix: None
      },
      TokenKind::Str => ParseRule {
        prec: Precedence::None,
        prefix: Some(|comp: &mut Compiler| { comp.string() }),
        infix: None,
      },
      TokenKind::Num => ParseRule {
        prec: Precedence::None,
        prefix: Some(|comp: &mut Compiler| { comp.number() }),
        infix: None,
      },
      TokenKind::Identifier => ParseRule {
        prec: Precedence::None,
        prefix: Some(|comp: &mut Compiler| { comp.variable() }),
        infix: None,
      },
      TokenKind::Dot => ParseRule {
        prec: Precedence::Func,
        prefix: None,
        infix: Some(|comp: &mut Compiler| { comp.member() }),
      },
      TokenKind::EqualEqual => ParseRule {
        prec: Precedence::Equal,
        prefix: None,
        infix: Some(|comp: &mut Compiler| { comp.binary() }),
      },
      TokenKind::LSquare => ParseRule {
        prec: Precedence::Func,
        prefix: None,
        infix: Some(|comp: &mut Compiler| { comp.computed_member() }),
      },
      TokenKind::RAngle => ParseRule {
        prec: Precedence::Comparison,
        prefix: None,
        infix: Some(|comp: &mut Compiler| { comp.binary() }),
      },
      TokenKind::Plus => ParseRule {
        prec: Precedence::Addition,
        prefix: None,
        infix: Some(|comp: &mut Compiler| { comp.binary() }),
      },
      TokenKind::Minus => ParseRule {
        prec: Precedence::Addition,
        prefix: None,
        infix: Some(|comp: &mut Compiler| { comp.binary() }),
      },
      TokenKind::Star => ParseRule {
        prec: Precedence::Multiplication,
        prefix: None,
        infix: Some(|comp: &mut Compiler| { comp.binary() }),
      },
      TokenKind::Slash => ParseRule {
        prec: Precedence::Multiplication,
        prefix: None,
        infix: Some(|comp: &mut Compiler| { comp.binary() }),
      },
      _ => ParseRule {
        prec: Precedence::None,
        prefix: None,
        infix: None
      },
    }
  }

  // parsing utils
  fn advance(&mut self) {
    let t = self.lexer.next_token();

    match t.kind {
      TokenKind::Error => self.fatal(format!("error on line {}: {}", t.line, t.str.unwrap())),
      _ => {
        self.prev = self.current.clone();
        self.current = t;
      }
    }

  }

  fn consume(&mut self, kind: TokenKind) {
    if self.current.kind != kind {
      self.fatal(format!("unexpected token {} expected {}", self.current, kind));
    }
    self.advance();
  }

  fn fatal(&self, message: String) {
    panic!("{}", message);
  }

  // opcodes
  fn emit(&mut self, opcode: OpCode) {
    self.output.push(opcode);
  }

  // grammar
  fn expression(&mut self, prec: Precedence) {
    let prefix_rule = self.get_rule(self.current.kind);
    if prefix_rule.prefix.is_none() {
      self.fatal(format!("unexpected prefix {}", self.current));
    }
    prefix_rule.prefix.unwrap()(self);

    while prec <= self.get_rule(self.current.kind).prec {
      let infix_rule = self.get_rule(self.current.kind);
      if infix_rule.infix.is_none() {
        self.fatal(format!("unexpected infix {}", self.current));
      }
      infix_rule.infix.unwrap()(self);
    }
  }

  fn statement(&mut self) {
    match self.current.kind {
      TokenKind::Print => {
        self.consume(TokenKind::Print);
        self.expression(Precedence::Assignment);
        self.emit(OpCode::Print);
      },
      TokenKind::Identifier => {
        self.variable();
      },
      _ => {
        self.fatal(format!("unexpected token '{}' expected a statement", self.current));
      },
    }
  }

  fn field(&mut self) {
    self.consume(TokenKind::Dollar);
    // TODO $name etc
    self.emit(OpCode::GetField(String::from("")));
  }

  fn binary(&mut self) {
    let token = self.current.clone();
    self.advance();
    self.expression(Precedence::Assignment);
    match token.kind {
      TokenKind::EqualEqual => self.emit(OpCode::Equal),
      TokenKind::RAngle => self.emit(OpCode::Greater),
      TokenKind::Plus => self.emit(OpCode::Add),
      TokenKind::Minus => self.emit(OpCode::Subtract),
      TokenKind::Star => self.emit(OpCode::Multiply),
      TokenKind::Slash => self.emit(OpCode::Divide),
      _ => self.fatal(format!("unknown operator {}", token.kind)),
    }
  }

  fn variable(&mut self) {
    self.consume(TokenKind::Identifier);
    let token = self.prev.clone();
    if self.current.kind == TokenKind::Equal {
      // assignment
      self.consume(TokenKind::Equal);
      self.expression(Precedence::Assignment);
      self.emit(OpCode::SetGlobal(token.str.unwrap()));
    } else {
      self.emit(OpCode::GetGlobal(token.str.unwrap()));
    }
  }

  fn member(&mut self) {
    self.consume(TokenKind::Dot);
    self.consume(TokenKind::Identifier);
    let token = self.prev.clone();
    self.emit(OpCode::PushImmediate(Value::Str(token.str.unwrap())));
    self.emit(OpCode::GetMember);
  }

  fn computed_member(&mut self) {
    self.consume(TokenKind::LSquare);
    self.expression(Precedence::Assignment);
    self.consume(TokenKind::RSquare);
    self.emit(OpCode::GetMember);
  }

  fn string(&mut self) {
    self.consume(TokenKind::Str);
    let token = self.prev.clone();
    self.emit(OpCode::PushImmediate(Value::Str(token.str.unwrap())));
  }

  fn number(&mut self) {
    self.consume(TokenKind::Num);
    let num: f64 = self.prev.clone().str.unwrap().parse().unwrap();
    self.emit(OpCode::PushImmediate(Value::Num(num)));
  }

  fn compile_rule(&mut self) -> JqaRule {
    let mut rule_kind = JqaRuleKind::Match;

    match self.current.kind {
      // no pattern
      TokenKind::LCurly => (),
      // begin/end
      TokenKind::Begin => {
        rule_kind = JqaRuleKind::Begin;
        self.consume(TokenKind::Begin);
      },
      TokenKind::End => {
        rule_kind = JqaRuleKind::End;
        self.consume(TokenKind::End);
      },
      // pattern
      _ => self.expression(Precedence::Assignment),
    }

    let pattern = self.output.clone();
    self.output.clear();

    self.consume(TokenKind::LCurly);
    while self.current.kind != TokenKind::RCurly {
      self.statement();
      if self.current.kind != TokenKind::RCurly {
        self.consume(TokenKind::Semicolon);
      }
    }
    self.consume(TokenKind::RCurly);
    let body = self.output.clone();
    self.output.clear();

    JqaRule { pattern, body, kind: rule_kind }
  }

  pub fn compile_expression(&mut self) -> Vec<OpCode> {
    self.advance();
    self.expression(Precedence::Assignment);
    return self.output.clone();
  }

  pub fn compile_rules(&mut self) -> Vec<JqaRule> {
    // prime the lexer
    self.advance();
    let mut rules = Vec::new();

    while self.current.kind != TokenKind::EOF {
      let rule = self.compile_rule();
      rules.push(rule);
    }

    return rules;
  }
}