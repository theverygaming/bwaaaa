package lexer

import "strconv"
import "unicode"
import "unicode/utf8"
import "errors"
import "io"
import "log"

type TokenType int

const (
	TokenError TokenType = iota
	TokenEOF
	// operators
	TokenPlus
	TokenMinus
	TokenMul
	TokenDiv
	TokenEquals
	TokenEqualsEquals
	TokenExclamation
	TokenExclamationEquals
	TokenQuestion
	TokenDot
	TokenComma
	TokenLt
	TokenGt
	// symbols
	TokenAt
	TokenSemicolon
	TokenColon
	TokenLeftParen
	TokenRightParen
	TokenLeftBrace
	TokenRightBrace
	TokenLeftBracket
	TokenRightBracket
	// basic types
	TokenInt
	TokenFloat
	TokenString
	TokenPathString
	// Keywords/Identifiers
	TokenIdentifier
	TokenKwTrue
	TokenKwFalse
	TokenKwNull
	TokenKwIf
	TokenKwThen
	TokenKwElse
	TokenKwInherit
	TokenKwWith
	TokenKwRec
	TokenKwLet
	TokenKwIn
)

type Token struct {
	Type TokenType
	Value any
}

func (i Token) AsString() string {
	if s, ok := i.Value.(string); ok {
		return s
	}
	return ""
}

func (i Token) AsInt() int64 {
	if s, ok := i.Value.(int64); ok {
		return s
	}
	return 0
}

func (i Token) AsFloat() float64 {
	if s, ok := i.Value.(float64); ok {
		return s
	}
	return 0.0
}

func (i Token) AsError() error {
	if s, ok := i.Value.(error); ok {
		return s
	}
	return errors.New("???")
}

func (i Token) String() string {
	switch i.Type {
	case TokenError:
		return "lexer error: " + i.AsError().Error()
	case TokenEOF:
		return "EOF!"
	case TokenPlus:
		return "+"
	case TokenMinus:
		return "-"
	case TokenMul:
		return "*"
	case TokenDiv:
		return "/"
	case TokenEquals:
		return "="
	case TokenEqualsEquals:
		return "=="
	case TokenExclamation:
		return "!"
	case TokenExclamationEquals:
		return "!="
	case TokenQuestion:
		return "?"
	case TokenDot:
		return "."
	case TokenComma:
		return ","
	case TokenLt:
		return "<"
	case TokenGt:
		return ">"
	case TokenAt:
		return "@"
	case TokenSemicolon:
		return ";"
	case TokenColon:
		return ":"
	case TokenLeftParen:
		return "("
	case TokenRightParen:
		return ")"
	case TokenLeftBrace:
		return "{"
	case TokenRightBrace:
		return "}"
	case TokenLeftBracket:
		return "["
	case TokenRightBracket:
		return "]"
	case TokenInt:
		return strconv.FormatInt(i.AsInt(), 10)
	case TokenFloat:
		return strconv.FormatFloat(i.AsFloat(), 'f', -1, 64)
	case TokenString:
		return "\"" + i.AsString() + "\""
	case TokenPathString:
		return "'" + i.AsString() + "'"
	case TokenIdentifier:
		return i.AsString()
	case TokenKwTrue:
		return "true"
	case TokenKwFalse:
		return "false"
	case TokenKwNull:
		return "null"
	case TokenKwIf:
		return "if"
	case TokenKwThen:
		return "then"
	case TokenKwElse:
		return "else"
	case TokenKwInherit:
		return "inherit"
	case TokenKwWith:
		return "with"
	case TokenKwRec:
		return "rec"
	case TokenKwLet:
		return "let"
	case TokenKwIn:
		return "in"
	default:
		return "???"
	}
}

type Lexer struct {
	input string
	pos int // offset in string
	width int // width of current rune
	futureToken Token // next token
}

func New(input string) *Lexer {
	lex := Lexer{input: input, pos: 0}
	lex.Next() // to initialize futureToken so the first Peek or Next will return something
	return &lex
}

func (lex *Lexer) Peek() Token {
	return lex.futureToken
}

func (lex *Lexer) Next() Token {
	current := lex.futureToken
	lex.futureToken = lex.nextToken()
	return current
}

func (lex *Lexer) nextToken() Token {
	err := lex.skipCommentsAndWhitespace()
	if err != nil {
		if errors.Is(err, io.EOF) {
			return Token{Type: TokenEOF, Value: nil}
		}
		return Token{Type: TokenError, Value: err}
	}
	r, err := lex.next()
	if err != nil {
		if errors.Is(err, io.EOF) {
			return Token{Type: TokenEOF, Value: nil}
		}
		return Token{Type: TokenError, Value: err}
	}

	// single character stuff
	switch r {
	case '+':
		return Token{Type: TokenPlus, Value: nil}
	case '-':
		return Token{Type: TokenMinus, Value: nil}
	case '*':
		return Token{Type: TokenMul, Value: nil}
	case '/':
		return Token{Type: TokenDiv, Value: nil}
	case '?':
		return Token{Type: TokenQuestion, Value: nil}
	case '.':
		return Token{Type: TokenDot, Value: nil}
	case ',':
		return Token{Type: TokenComma, Value: nil}
	case '@':
		return Token{Type: TokenAt, Value: nil}
	case ';':
		return Token{Type: TokenSemicolon, Value: nil}
	case '<':
		return Token{Type: TokenLt, Value: nil}
	case '>':
		return Token{Type: TokenGt, Value: nil}
	case ':':
		return Token{Type: TokenColon, Value: nil}
	case '(':
		return Token{Type: TokenLeftParen, Value: nil}
	case ')':
		return Token{Type: TokenRightParen, Value: nil}
	case '{':
		return Token{Type: TokenLeftBrace, Value: nil}
	case '}':
		return Token{Type: TokenRightBrace, Value: nil}
	case '[':
		return Token{Type: TokenLeftBracket, Value: nil}
	case ']':
		return Token{Type: TokenRightBracket, Value: nil}
	}
	lex.back()

	// FIXME: this is cursed. The error handling is a damn mess everywhere :c
	// =, ==, ! and !=
	start := lex.pos
	r1, err := lex.next()
	if err != nil {
		if errors.Is(err, io.EOF) {
			return Token{Type: TokenEOF, Value: nil}
		}
		return Token{Type: TokenError, Value: err}
	}
	startr2 := lex.pos
	r2, err := lex.next()
	if err != nil {
		if errors.Is(err, io.EOF) {
			return Token{Type: TokenEOF, Value: nil}
		}
		return Token{Type: TokenError, Value: err}
	}
	if r1 == '=' {
		if r2 == '=' {
			return Token{Type: TokenEqualsEquals, Value: nil}
		}
		lex.pos = startr2
		return Token{Type: TokenEquals, Value: nil}
	}
	if r1 == '!' {
		if r2 == '=' {
			return Token{Type: TokenExclamationEquals, Value: nil}
		}
		lex.pos = startr2
		return Token{Type: TokenExclamation, Value: nil}
	}
	lex.pos = start

	// prefix stuff
	switch r {
	case '"':
		return lex.scanString(false)
	case '\'':
		return lex.scanString(true)
	}

	if unicode.IsDigit(r) {
		return lex.scanNumber()
	} else if isIdentifierRune(r) {
		return lex.scanIdentifier()
	}

	return Token{Type: TokenError, Value: errors.New("unexpected token: " + string(r))}
}

func (lex *Lexer) scanNumber() Token {
	start := lex.pos
	isFloat := false
	isHex := false
	gotEOF := false
	for {
		r, err := lex.next()
		if err != nil {
			if errors.Is(err, io.EOF) {
				// on EOF the number just ends here and we go with it!
				gotEOF = true
				break
			}
			return Token{Type: TokenError, Value: err}
		}
		// float
		if r == '.' {
			// extra dot or hex? Impossible.
			if isFloat {
				return Token{Type: TokenError, Value: errors.New("more than one dot float")}
			}
			if isHex {
				return Token{Type: TokenError, Value: errors.New("hex & float number???")}
			}
			isFloat = true
			continue
		}
		// hex
		if r == 'x' {
			// needs to start with 0x for hex
			if !(lex.pos - start == 2 && lex.input[start] == '0') {
				return Token{Type: TokenError, Value: errors.New("invalid hex number")}
			}
			isHex = true
			continue
		}
		if !(unicode.IsDigit(r) || (isHex && ((r >= 'a' && r <= 'f') || (r >= 'A' && r <= 'F')))) {
			break
		}
	}
	if !gotEOF {
		lex.back()
	}
	n := lex.pos - start
	if isFloat {
		val, err := strconv.ParseFloat(lex.input[start:start+n], 64)
		if err != nil {
			return Token{Type: TokenError, Value: err}
		}
		return Token{Type: TokenFloat, Value: val}
	} else {
		base := 10
		offset := 0
		if isHex {
			base = 16
			offset = 2
		}
		val, err := strconv.ParseInt(lex.input[start+offset:start+n], base, 64)
		if err != nil {
			return Token{Type: TokenError, Value: err}
		}
		return Token{Type: TokenInt, Value: val}
	}
}

func (lex *Lexer) scanIdentifier() Token {
	start := lex.pos
	gotEOF := false
	for {
		r, err := lex.next()
		if err != nil {
			if errors.Is(err, io.EOF) {
				// on EOF the indentifier just ends here and we go with it!
				gotEOF = true
				break
			}
			return Token{Type: TokenError, Value: err}
		}

		if !isIdentifierRune(r) {
			break
		}
	}
	if !gotEOF {
		lex.back()
	}
	n := lex.pos - start
	ident := lex.input[start:start+n]

	switch ident {
	case "true":
		return Token{Type: TokenKwTrue, Value: nil}
	case "false":
		return Token{Type: TokenKwFalse, Value: nil}
	case "null":
		return Token{Type: TokenKwNull, Value: nil}
	case "if":
		return Token{Type: TokenKwIf, Value: nil}
	case "then":
		return Token{Type: TokenKwThen, Value: nil}
	case "else":
		return Token{Type: TokenKwElse, Value: nil}
	case "inherit":
		return Token{Type: TokenKwInherit, Value: nil}
	case "with":
		return Token{Type: TokenKwWith, Value: nil}
	case "rec":
		return Token{Type: TokenKwRec, Value: nil}
	case "let":
		return Token{Type: TokenKwLet, Value: nil}
	case "in":
		return Token{Type: TokenKwIn, Value: nil}
	}

	return Token{Type: TokenIdentifier, Value: ident}
}

func (lex *Lexer) scanString(isPath bool) Token {
	quotechar := '"'
	if isPath {
		quotechar = '\''
	}
	start := lex.pos
	result := ""
	escaped := false
	loop:
	for {
		r, err := lex.next()
		if err != nil {
			if errors.Is(err, io.EOF) {
				return Token{Type: TokenEOF, Value: nil}
			}
			return Token{Type: TokenError, Value: err}
		}

		if escaped {
			switch r {
			case '\\':
				result += string('\\')
			case 'n':
				result += string('\n')
			case quotechar:
				result += string(quotechar)
			default:
				result += string(r)
				log.Println("unknown escape sequence \\" + string(r) + " - taking the char after the backslash as is")
			}
			escaped = false
			continue
		}

		switch r {
		case '\\':
			escaped = true
			continue
		case quotechar:
			// start quote
			if lex.pos - start == 1 {
				continue
			}
			// end quote
			break loop
		}

		// normal char
		result += string(r)
	}

	if isPath {
		return Token{Type: TokenPathString, Value: result}
	} else {
		return Token{Type: TokenString, Value: result}
	}
}

func (lex *Lexer) skipCommentsAndWhitespace() error {
	for {
		err := lex.skipWhitespace()
		if err != nil {
			return err
		}
		r, err := lex.next()
		if err != nil {
			return err
		}
		if r == '#' {
			err := lex.skipLine()
			if err != nil {
				return err
			}
			continue
		}
		break
	}
	lex.back()
	return nil
}

func (lex *Lexer) skipWhitespace() error {
	for {
		r, err := lex.next()
		if err != nil {
			return err
		}
		if !unicode.IsSpace(r) {
			break
		}
	}
	lex.back()
	return nil
}

func (lex *Lexer) skipLine() error {
	for {
		r, err := lex.next()
		if err != nil {
			return err
		}
		if r == '\n' {
			break
		}
	}
	return nil
}

func (lex *Lexer) next() (rune, error) {
	if lex.pos >= len(lex.input) {
		return 0, io.EOF
	}
	val, width := utf8.DecodeRuneInString(lex.input[lex.pos:])
	lex.pos += width
	lex.width = width
	return val, nil
}

func (lex *Lexer) back() {
	lex.pos -= lex.width
}

func isIdentifierRune(r rune) bool {
	return unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_'
}
