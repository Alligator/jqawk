package lang

import (
	"fmt"
	"unicode"
)

type TokenTag uint8

//go:generate stringer -type=TokenTag -linecomment
const (
	EOF TokenTag = iota
	Error
	Ident
	Str
	Regex
	Num
	Begin
	End
	Print
	Function
	Return
	If
	Else
	LCurly        // {
	RCurly        // }
	LSquare       // [
	RSquare       // ]
	LParen        // (
	RParen        // )
	LessThan      // <
	GreaterThan   // >
	Dollar        // $
	Comma         // ,
	Dot           // .
	Equal         // =
	EqualEqual    // ==
	BangEqual     // !=
	LessEqual     // <=
	GreaterEqual  // >=
	SemiColon     // ;
	Plus          // +
	Minus         // -
	Multiply      // *
	Divide        // /
	PlusEqual     // +=
	MinusEqual    // -=
	MultiplyEqual // *=
	DivideEqual   // /=
	Tilde         // ~
	BangTilde     // !~
	AmpAmp        // &&
	PipePipe      // ||
)

type Token struct {
	Tag TokenTag
	Pos int
	Len int
}

type Lexer struct {
	src        string
	pos        int
	line       int
	tokenStart int
}

func NewLexer(src string) Lexer {
	return Lexer{
		src:        src,
		pos:        0,
		line:       1,
		tokenStart: 0,
	}
}

func (l *Lexer) atEnd() bool {
	return l.pos >= len(l.src)
}

func (l *Lexer) simpleToken(tag TokenTag) Token {
	return Token{tag, l.tokenStart, 0}
}

func (l *Lexer) errorToken() Token {
	return Token{Error, l.tokenStart, 0}
}

func (l *Lexer) stringToken(tag TokenTag, length int) Token {
	return Token{tag, l.tokenStart, length}
}

func (l *Lexer) advance() byte {
	if !l.atEnd() {
		l.pos++
	}
	return l.src[l.pos-1]
}

func (l *Lexer) peek() byte {
	return l.src[l.pos]
}

func (l *Lexer) skipWhitespace() {
	for !l.atEnd() {
		switch l.peek() {
		case ' ', '\n', '\r', '\t':
			l.advance()
		case '#':
			for !l.atEnd() && l.peek() != '\n' {
				l.advance()
			}
		default:
			return
		}
	}
}

func (l *Lexer) identifier() Token {
	for !l.atEnd() {
		r := rune(l.peek())
		if r == '_' || unicode.IsLetter(r) || unicode.IsDigit(r) {
			l.advance()
		} else {
			break
		}
	}
	str := l.src[l.tokenStart:l.pos]
	switch str {
	case "BEGIN":
		return l.simpleToken(Begin)
	case "END":
		return l.simpleToken(End)
	case "print":
		return l.simpleToken(Print)
	case "$":
		return l.simpleToken(Dollar)
	case "function":
		return l.simpleToken(Function)
	case "return":
		return l.simpleToken(Return)
	case "if":
		return l.simpleToken(If)
	case "else":
		return l.simpleToken(Else)
	default:
		return l.stringToken(Ident, l.pos-l.tokenStart)
	}
}

func (l *Lexer) number() Token {
	for !l.atEnd() {
		r := rune(l.peek())
		if unicode.IsDigit(r) {
			l.advance()
		} else {
			break
		}
	}
	return l.stringToken(Num, l.pos-l.tokenStart)
}

func (l *Lexer) string(quoteChar byte) (Token, error) {
	for l.peek() != quoteChar {
		if l.atEnd() {
			return l.errorToken(), fmt.Errorf("unexpected EOF while reading string")
		}
		l.advance()
	}
	l.advance()
	l.tokenStart++ // skip over the opening quote
	return l.stringToken(Str, l.pos-l.tokenStart-1), nil
}

// the parser calls this when it finds a '/' in prefix position
func (l *Lexer) regex() (Token, error) {
	for l.peek() != '/' {
		if l.atEnd() {
			return l.errorToken(), fmt.Errorf("unexpected EOF while reading regex")
		}
		l.advance()
	}
	l.advance()
	l.tokenStart++
	return l.stringToken(Regex, l.pos-l.tokenStart-1), nil
}

func (l *Lexer) GetString(token *Token) string {
	return l.src[token.Pos : token.Pos+token.Len]
}

func (l *Lexer) Next() (Token, error) {
	l.skipWhitespace()
	if l.atEnd() {
		return l.simpleToken(EOF), nil
	}

	c := l.peek()
	r := rune(c)
	l.tokenStart = l.pos

	if c == '$' {
		l.pos++
		return l.identifier(), nil
	}

	if unicode.IsDigit(r) {
		return l.number(), nil
	}

	if unicode.IsLetter(r) {
		return l.identifier(), nil
	}

	l.advance()

	switch c {
	case '{':
		return l.simpleToken(LCurly), nil
	case '}':
		return l.simpleToken(RCurly), nil
	case '[':
		return l.simpleToken(LSquare), nil
	case ']':
		return l.simpleToken(RSquare), nil
	case '(':
		return l.simpleToken(LParen), nil
	case ')':
		return l.simpleToken(RParen), nil
	case ',':
		return l.simpleToken(Comma), nil
	case '.':
		return l.simpleToken(Dot), nil
	case ';':
		return l.simpleToken(SemiColon), nil
	case '~':
		return l.simpleToken(Tilde), nil
	case '<':
		if l.peek() == '=' {
			l.advance()
			return l.simpleToken(LessEqual), nil
		}
		return l.simpleToken(LessThan), nil
	case '>':
		if l.peek() == '=' {
			l.advance()
			return l.simpleToken(GreaterEqual), nil
		}
		return l.simpleToken(GreaterThan), nil
	case '+':
		if l.peek() == '=' {
			l.advance()
			return l.simpleToken(PlusEqual), nil
		}
		return l.simpleToken(Plus), nil
	case '-':
		if l.peek() == '=' {
			l.advance()
			return l.simpleToken(MinusEqual), nil
		}
		return l.simpleToken(Minus), nil
	case '*':
		if l.peek() == '=' {
			l.advance()
			return l.simpleToken(MultiplyEqual), nil
		}
		return l.simpleToken(Multiply), nil
	case '/':
		if l.peek() == '=' {
			l.advance()
			return l.simpleToken(DivideEqual), nil
		}
		return l.simpleToken(Divide), nil
	case '=':
		if l.peek() == '=' {
			l.advance()
			return l.simpleToken(EqualEqual), nil
		}
		return l.simpleToken(Equal), nil
	case '!':
		switch l.peek() {
		case '=':
			l.advance()
			return l.simpleToken(BangEqual), nil
		case '~':
			l.advance()
			return l.simpleToken(BangTilde), nil
		}
	case '&':
		if l.peek() == '&' {
			l.advance()
			return l.simpleToken(AmpAmp), nil
		}
	case '|':
		if l.peek() == '|' {
			l.advance()
			return l.simpleToken(PipePipe), nil
		}
	case '\'', '"':
		return l.string(c)
	}
	return l.errorToken(), fmt.Errorf("unexpected character %q", c)
}
