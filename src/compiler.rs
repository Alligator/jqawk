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
  Logical,
  Equal,
  Comparison,
  Addition,
  Multiplication,
  Func,
}

struct ParseRule {
  prec: Precedence,
  infix: Option<fn(&mut Compiler) -> Result<(), SyntaxError>>,
  prefix: Option<fn(&mut Compiler) -> Result<(), SyntaxError>>,
}

#[derive(Debug)]
pub struct SyntaxError {
  pub msg: String,
  pub line: usize,
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
      TokenKind::Equal => ParseRule {
        prec: Precedence::Assignment,
        prefix: None,
        infix: Some(|comp: &mut Compiler| { comp.assign() }),
      },
      TokenKind::EqualEqual => ParseRule {
        prec: Precedence::Equal,
        prefix: None,
        infix: Some(|comp: &mut Compiler| { comp.binary() }),
      },
      TokenKind::And => ParseRule {
        prec: Precedence::Logical,
        prefix: None,
        infix: Some(|comp: &mut Compiler| { comp.binary() }),
      },
      TokenKind::Or => ParseRule {
        prec: Precedence::Logical,
        prefix: None,
        infix: Some(|comp: &mut Compiler| { comp.binary() }),
      },
      TokenKind::Tilde => ParseRule {
        prec: Precedence::Equal,
        prefix: None,
        infix: Some(|comp: &mut Compiler| { comp.binary() }),
      },
      TokenKind::BangTilde => ParseRule {
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
        prefix: Some(|comp: &mut Compiler| { comp.regex() }),
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
  fn advance(&mut self) -> Result<(), SyntaxError> {
    let t = self.lexer.next_token();

    match t.kind {
      TokenKind::Error => {
        return Err(SyntaxError {
          msg: t.str.unwrap(),
          line: t.line,
        });
      },
      _ => {
        self.prev = self.current.clone();
        self.current = t;
        return Ok(());
      }
    }

  }

  fn consume(&mut self, kind: TokenKind) -> Result<(), SyntaxError> {
    if self.current.kind != kind {
      return Err(SyntaxError {
        msg: format!("unexpected token {} expected {}", self.current, kind),
        line: self.current.line,
      });
    }
    return self.advance();
  }

  fn error(&self, message: String, line: usize) -> Result<(), SyntaxError> {
    return Err(SyntaxError {
      msg: message,
      line: line,
    });
  }

  // opcodes
  fn emit(&mut self, opcode: OpCode) {
    self.output.push(opcode);
  }

  // grammar
  fn expression(&mut self, prec: Precedence) -> Result<(), SyntaxError> {
    let prefix_rule = self.get_rule(self.current.kind);
    if prefix_rule.prefix.is_none() {
      return self.error(format!("unexpected prefix {}", self.current), self.current.line);
    }
    prefix_rule.prefix.unwrap()(self)?;

    while prec <= self.get_rule(self.current.kind).prec {
      let infix_rule = self.get_rule(self.current.kind);
      if infix_rule.infix.is_none() {
        return self.error(format!("unexpected infix {}", self.current), self.current.line);
      }
      infix_rule.infix.unwrap()(self)?;
    }
    return Ok(());
  }

  fn statement(&mut self) -> Result<(), SyntaxError> {
    match self.current.kind {
      TokenKind::Print => {
        self.consume(TokenKind::Print)?;
        let mut arg_count = 0;
        while !self.at_statement_end() {
          self.expression(Precedence::Assignment)?;
          arg_count += 1;
          if self.current.kind == TokenKind::Comma {
            self.consume(TokenKind::Comma)?;
          } else {
            break;
          }
        }
        self.emit(OpCode::Print(arg_count));
        return Ok(());
      },
      _ => return self.expression(Precedence::Assignment)
    }
  }

  fn at_statement_end(&self) -> bool {
    match self.current.kind {
      TokenKind::Semicolon | TokenKind::RCurly => true,
      _ => false,
    }
  }

  fn field(&mut self) -> Result<(), SyntaxError> {
    self.consume(TokenKind::Dollar)?;
    // TODO $name etc
    self.emit(OpCode::GetField(String::from("")));
    return Ok(());
  }

  fn binary(&mut self) -> Result<(), SyntaxError> {
    let token = self.current.clone();
    let prec = self.get_rule(token.kind).prec;
    self.advance()?;
    self.expression(prec)?;
    match token.kind {
      TokenKind::EqualEqual => self.emit(OpCode::Equal),
      TokenKind::And => self.emit(OpCode::And),
      TokenKind::Or => self.emit(OpCode::Or),
      TokenKind::RAngle => self.emit(OpCode::Greater),
      TokenKind::Plus => self.emit(OpCode::Add),
      TokenKind::Minus => self.emit(OpCode::Subtract),
      TokenKind::Star => self.emit(OpCode::Multiply),
      TokenKind::Slash => self.emit(OpCode::Divide),
      TokenKind::Tilde => self.emit(OpCode::Match),
      TokenKind::BangTilde => {
        self.emit(OpCode::Match);
        self.emit(OpCode::Negate);
      }
      _ => {
        return Err(SyntaxError {
          msg: format!("unknown operator {}", token.kind),
          line: token.line,
        });
      }
    }
    return Ok(());
  }

  fn variable(&mut self) -> Result<(), SyntaxError> {
    self.consume(TokenKind::Identifier)?;
    let token = self.prev.clone();
    self.emit(OpCode::GetGlobal(token.str.unwrap()));
    return Ok(());
  }

  fn member(&mut self) -> Result<(), SyntaxError> {
    self.consume(TokenKind::Dot)?;
    self.consume(TokenKind::Identifier)?;
    let token = self.prev.clone();
    self.emit(OpCode::PushImmediate(Value::Str(token.str.unwrap())));
    self.emit(OpCode::GetMember);
    return Ok(());
  }

  fn computed_member(&mut self) -> Result<(), SyntaxError> {
    self.consume(TokenKind::LSquare)?;
    self.expression(Precedence::Assignment)?;
    self.consume(TokenKind::RSquare)?;
    self.emit(OpCode::GetMember);
    return Ok(());
  }

  fn assign(&mut self) -> Result<(), SyntaxError> {
    self.consume(TokenKind::Equal)?;

    // crazy assignment handling
    // this is a one pass compiler, when we get here, the lhs of this assignment
    // has already been compiled to a Get* opcode. we need it to be a set.  to
    // do this, we stash the get opcode, compile the rhs, then flip the get to a
    // set.
    let last_opcode = self.output.pop().unwrap();

    self.expression(Precedence::Assignment)?;

    let new_opcode = match last_opcode {
      OpCode::GetGlobal(s) => OpCode::SetGlobal(s.clone()),
      _ => panic!("expected a Get opcode before assign"),
    };
    self.emit(new_opcode);

    return Ok(());
  }

  fn string(&mut self) -> Result<(), SyntaxError> {
    self.consume(TokenKind::Str)?;
    let token = self.prev.clone();
    self.emit(OpCode::PushImmediate(Value::Str(token.str.unwrap())));
    return Ok(());
  }

  fn regex(&mut self) -> Result<(), SyntaxError> {
    let t = self.lexer.read_regex();

    match t.kind {
      TokenKind::Error => {
        return self.error(format!("error on line {}: {}", t.line, t.str.unwrap()), t.line);
      },
      _ => {
        self.prev = self.current.clone();
        self.current = t;
      }
    }
    self.advance()?;
    let token = self.prev.clone();
    self.emit(OpCode::PushImmediate(Value::Regex(token.str.unwrap())));
    return Ok(());
  }

  fn number(&mut self) -> Result<(), SyntaxError> {
    self.consume(TokenKind::Num)?;
    let num: f64 = self.prev.clone().str.unwrap().parse().unwrap();
    self.emit(OpCode::PushImmediate(Value::Num(num)));
    return Ok(());
  }

  fn compile_rule(&mut self) -> Result<JqaRule, SyntaxError> {
    let mut rule_kind = JqaRuleKind::Match;

    match self.current.kind {
      // no pattern
      TokenKind::LCurly => (),
      // begin/end
      TokenKind::Begin => {
        rule_kind = JqaRuleKind::Begin;
        self.consume(TokenKind::Begin)?;
      },
      TokenKind::End => {
        rule_kind = JqaRuleKind::End;
        self.consume(TokenKind::End)?;
      },
      // pattern
      _ => {
        self.expression(Precedence::Assignment)?;
      },
    }

    let pattern = self.output.clone();
    self.output.clear();

    if self.current.kind != TokenKind::LCurly {
      self.emit(OpCode::Print(0));
    } else {
      self.consume(TokenKind::LCurly)?;
      while self.current.kind != TokenKind::RCurly {
        self.statement()?;
        if self.current.kind != TokenKind::RCurly {
          self.consume(TokenKind::Semicolon)?;
        }
      }
      self.consume(TokenKind::RCurly)?;
    }
    let body = self.output.clone();
    self.output.clear();

    return Ok(JqaRule { pattern, body, kind: rule_kind });
  }

  pub fn compile_expression(&mut self) -> Result<Vec<OpCode>, SyntaxError> {
    self.advance()?;
    self.expression(Precedence::Assignment)?;
    return Ok(self.output.clone());
  }

  pub fn compile_rules(&mut self) -> Result<Vec<JqaRule>, SyntaxError> {
    // prime the lexer
    self.advance()?;
    let mut rules = Vec::new();

    while self.current.kind != TokenKind::EOF {
      let rule = self.compile_rule()?;
      rules.push(rule);
    }

    return Ok(rules);
  }
}