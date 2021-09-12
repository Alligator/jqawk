use std::fmt;

#[derive(Copy, Clone, PartialEq, Debug)]
pub enum TokenKind {
    Dollar,
    Dot,
    EqualEqual,
    LCurly,
    RCurly,
    Semicolon,
    Str,
    Identifier,
    Print,
    Error, 
    EOF,
}

impl fmt::Display for TokenKind {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
      write!(f, "{}", match self {
        TokenKind::Dollar => "$",
        TokenKind::Dot => ".",
        TokenKind::EqualEqual => "==",
        TokenKind::LCurly => "{",
        TokenKind::RCurly => "}",
        TokenKind::Semicolon => ";",
        TokenKind::Print => "print",
        TokenKind::Str => "<string>",
        TokenKind::Identifier => "<identifier>",
        TokenKind::Error => "<error>",
        TokenKind::EOF => "<eof>",
      })
    }
}

#[derive(Clone, Debug)]
pub struct Token {
  pub kind: TokenKind,
  pub str: Option<String>,
}

impl fmt::Display for Token {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
      match self.kind {
        TokenKind::Str | TokenKind::Identifier => write!(f, "{}", self.str.as_ref().unwrap()),
        _ => write!(f, "{}", self.kind),
      }
    }
}

impl Token {
  pub fn simple(kind: TokenKind) -> Token {
    Token {
      kind: kind,
      str: None,
    }
  }

  pub fn str(kind: TokenKind, str: &str) -> Token {
    Token {
      kind: kind,
      str: Some(String::from(str)),
    }
  }

  pub fn err(message: String) -> Token {
    Token {
      kind: TokenKind::Error,
      str: Some(message),
    }
  }
}

#[derive(Debug)]
pub struct Lexer {
    src: String,
    pos: usize,
    token_start: usize,
    line: u64,
}

impl Lexer {
    pub fn new(src: &str) -> Lexer {
        Lexer {
            src: String::from(src),
            pos: 0,
            token_start: 0,
            line: 1,
        }
    }

    fn advance(&mut self) -> Option<char> {
        if self.pos <= self.src.len() {
            self.pos += 1
        }
        self.src.chars().nth(self.pos - 1)
    }
    fn peek(&mut self) -> Option<char> {
        self.src.chars().nth(self.pos)
    }

    fn skip_whitespace(&mut self) {
        loop {
            match self.peek() {
                Some(' ') => self.pos += 1,
                Some('\n') => {
                    self.pos += 1;
                    self.line += 1;
                }
                _ => break,
            }
        }
    }

    fn identifier(&mut self) -> Token {
        while self.peek().unwrap_or_default().is_ascii_alphabetic() {
            self.advance();
        }
        let ident = &self.src[self.token_start..self.pos];

        match ident {
          "print" => Token::simple(TokenKind::Print),
          _ => Token::str(TokenKind::Identifier, ident),
        }
    }

    fn string(&mut self) -> Token {
        loop {
            match self.peek() {
                Some('"') => break,
                Some(_) => { self.advance(); },
                None => return Token::err(String::from("unexpected EOF in string")),
            }
        }
        self.advance();
        let str_content = &self.src[self.token_start + 1 .. self.pos - 1];
        return Token::str(TokenKind::Str, str_content);
    }

    pub fn next_token(&mut self) -> Token {
        self.skip_whitespace();
        self.token_start = self.pos;

         let c = match self.advance() {
            Some(c) => c,
            None => return Token::simple(TokenKind::EOF),
        };

        if c.is_ascii_alphabetic() {
            return self.identifier();
        }

        if c == '"' {
            return self.string();
        }

        match c {
            '$' => return Token::simple(TokenKind::Dollar),
            '.' => return Token::simple(TokenKind::Dot),
            '{' => return Token::simple(TokenKind::LCurly),
            '}' => return Token::simple(TokenKind::RCurly),
            ';' => return Token::simple(TokenKind::Semicolon),
            '=' => {
                if self.peek() == Some('=') {
                    self.advance();
                    return Token::simple(TokenKind::EqualEqual);
                }
            }
            _ => (),
        }

        return Token::err(format!("unexpected character '{}'", c));
    }
}
