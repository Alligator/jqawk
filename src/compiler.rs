use crate::vm::{OpCode, Value};
use crate::lexer::{Lexer, Token, TokenKind};

pub struct Compiler {
  current: Token,
  prev: Token,
  lexer: Lexer,
  output: Vec<OpCode>,
}

#[derive(Clone, Debug)]
pub struct JqaRule {
  pub pattern: Vec<OpCode>,
  pub body: Vec<OpCode>,
}


#[derive(PartialOrd, PartialEq)]
enum Precedence {
  None = 0,
  Assignment,
  Func,
  Equal,
}

struct ParseRule {
  prec: Precedence,
  infix: Option<fn(&mut Compiler)>,
  prefix: Option<fn(&mut Compiler)>,
}


impl Compiler {
  pub fn new(lexer: Lexer) -> Compiler {
    Compiler {
      current: Token::simple(TokenKind::EOF),
      prev: Token::simple(TokenKind::EOF),
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
      TokenKind::Error => self.fatal(t.str.unwrap()),
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
      _ => {
        self.fatal(format!("unexpected token {} expected a statement", self.current.kind));
      },
    }

    self.consume(TokenKind::Semicolon);
  }

  fn field(&mut self) {
    self.consume(TokenKind::Dollar);
    // TODO $name etc
    self.emit(OpCode::GetField(String::from("")));
  }

  fn binary(&mut self) {
    match self.current.kind {
      TokenKind::EqualEqual => {
        self.consume(TokenKind::EqualEqual);
        self.expression(Precedence::Assignment);
        self.emit(OpCode::Equal);
      },
      _ => self.fatal(format!("unknown operator {}", self.current.kind)),
    }
  }

  fn member(&mut self) {
    self.consume(TokenKind::Dot);
    self.consume(TokenKind::Identifier);
    let token = self.prev.clone();
    self.emit(OpCode::PushImmediate(Value::Str(token.str.unwrap())));
    self.emit(OpCode::GetMember);
  }

  fn string(&mut self) {
    self.consume(TokenKind::Str);
    let token = self.prev.clone();
    self.emit(OpCode::PushImmediate(Value::Str(token.str.unwrap())));
  }

  fn compile_rule(&mut self) -> JqaRule {
    if self.current.kind != TokenKind::LCurly {
      self.expression(Precedence::Assignment);
    }

    let pattern = self.output.clone();
    self.output.clear();

    self.consume(TokenKind::LCurly);
    while self.current.kind != TokenKind::RCurly {
      self.statement();
    }
    self.consume(TokenKind::RCurly);
    let body = self.output.clone();
    self.output.clear();

    JqaRule { pattern, body }
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