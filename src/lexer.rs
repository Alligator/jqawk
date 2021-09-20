use std::fmt;

#[derive(Copy, Clone, PartialEq, Debug)]
pub enum TokenKind {
    Dollar,
    Dot,
    Plus,
    Minus,
    Star,
    Slash,
    Equal,
    EqualEqual,
    LCurly,
    RCurly,
    LSquare,
    RSquare,
    LAngle,
    RAngle,
    Comma,
    Semicolon,
    Str,
    Num,
    Identifier,
    Print,
    Begin,
    End,
    Error, 
    EOF,
}

impl fmt::Display for TokenKind {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
      write!(f, "{}", match self {
        TokenKind::Dollar => "$",
        TokenKind::Dot => ".",
        TokenKind::Plus => "+",
        TokenKind::Minus => "-",
        TokenKind::Star => "*",
        TokenKind::Slash => "/",
        TokenKind::Equal => "=",
        TokenKind::EqualEqual => "==",
        TokenKind::LCurly => "{",
        TokenKind::RCurly => "}",
        TokenKind::LSquare => "[",
        TokenKind::RSquare => "]",
        TokenKind::LAngle => "<",
        TokenKind::RAngle => ">",
        TokenKind::Comma => ",",
        TokenKind::Semicolon => ";",
        TokenKind::Print => "print",
        TokenKind::Str => "<string>",
        TokenKind::Num => "<num>",
        TokenKind::Identifier => "<identifier>",
        TokenKind::Begin => "BEGIN",
        TokenKind::End => "END",
        TokenKind::Error => "<error>",
        TokenKind::EOF => "<eof>",
      })
    }
}

#[derive(Clone, Debug)]
pub struct Token {
  pub kind: TokenKind,
  pub str: Option<String>,
  pub line: usize,
}

impl fmt::Display for Token {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
      match self.kind {
        TokenKind::Str | TokenKind::Identifier | TokenKind::Num =>
          write!(f, "{}", self.str.as_ref().unwrap()),
        _ => write!(f, "{}", self.kind),
      }
    }
}

impl Token {
  pub fn new(kind: TokenKind, line: usize) -> Token {
    Token {
      kind: kind,
      str: None,
      line,
    }
  }
}

#[derive(Debug)]
pub struct Lexer {
    src: String,
    pos: usize,
    token_start: usize,
    line: usize,
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

    fn simple_token(&self, kind: TokenKind) -> Token {
        Token {
            kind,
            str: None,
            line: self.line,
        }
    }

    fn str_token(&self, kind: TokenKind, str: &str) -> Token {
        Token {
            kind,
            str: Some(String::from(str)),
            line: self.line,
        }
    }

    fn err_token(&self, message: String) -> Token {
        Token {
            kind: TokenKind::Error,
            str: Some(message),
            line: self.line,
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
                Some(' ') | Some('\r') => self.pos += 1,
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
          "print" => self.simple_token(TokenKind::Print),
          "BEGIN" => self.simple_token(TokenKind::Begin),
          "END" => self.simple_token(TokenKind::End),
          _ => self.str_token(TokenKind::Identifier, ident),
        }
    }

    fn number(&mut self) -> Token {
        while self.peek().unwrap_or_default().is_ascii_digit() {
            self.advance();
        }
        let num = &self.src[self.token_start..self.pos];
        return self.str_token(TokenKind::Num, num);
    }

    fn string(&mut self) -> Token {
        loop {
            match self.peek() {
                Some('"') => break,
                Some(_) => { self.advance(); },
                None => return self.err_token(String::from("unexpected EOF in string")),
            }
        }
        self.advance();
        let str_content = &self.src[self.token_start + 1 .. self.pos - 1];
        return self.str_token(TokenKind::Str, str_content);
    }

    pub fn next_token(&mut self) -> Token {
        self.skip_whitespace();
        self.token_start = self.pos;

         let c = match self.advance() {
            Some(c) => c,
            None => return self.simple_token(TokenKind::EOF),
        };

        if c.is_ascii_alphabetic() {
            return self.identifier();
        }

        if c.is_ascii_digit() {
            return self.number();
        }

        if c == '"' {
            return self.string();
        }

        match c {
            '$' => return self.simple_token(TokenKind::Dollar),
            '.' => return self.simple_token(TokenKind::Dot),
            '+' => return self.simple_token(TokenKind::Plus),
            '-' => return self.simple_token(TokenKind::Minus),
            '*' => return self.simple_token(TokenKind::Star),
            '/' => return self.simple_token(TokenKind::Slash),
            '{' => return self.simple_token(TokenKind::LCurly),
            '}' => return self.simple_token(TokenKind::RCurly),
            '[' => return self.simple_token(TokenKind::LSquare),
            ']' => return self.simple_token(TokenKind::RSquare),
            '<' => return self.simple_token(TokenKind::LAngle),
            '>' => return self.simple_token(TokenKind::RAngle),
            ',' => return self.simple_token(TokenKind::Comma),
            ';' => return self.simple_token(TokenKind::Semicolon),
            '=' => {
                if self.peek() == Some('=') {
                    self.advance();
                    return self.simple_token(TokenKind::EqualEqual);
                }
                return self.simple_token(TokenKind::Equal);
            }
            _ => (),
        }

        return self.err_token(format!("unexpected character '{}'", c));
    }
}
