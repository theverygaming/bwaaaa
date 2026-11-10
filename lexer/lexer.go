package lexer

import "strconv"
import "unicode"
import "unicode/utf8"
import "errors"
import "io"
import "log"

type ItemType int

const (
	ItemError ItemType = iota
	ItemEOF
	// operators
	ItemPlus
	ItemMinus
	ItemMul
	ItemDiv
	ItemEquals
	ItemEqualsEquals
	ItemExclamation
	ItemExclamationEquals
	ItemQuestion
	ItemDot
	ItemComma
	ItemLt
	ItemGt
	// symbols
	ItemAt
	ItemSemicolon
	ItemColon
	ItemLeftParen
	ItemRightParen
	ItemLeftBrace
	ItemRightBrace
	ItemLeftBracket
	ItemRightBracket
	// basic types
	ItemInt
	ItemFloat
	ItemString
	ItemPathString
	// Keywords/Identifiers
	ItemIdentifier
	ItemKwTrue
	ItemKwFalse
	ItemKwNull
	ItemKwIf
	ItemKwThen
	ItemKwElse
	ItemKwInherit
	ItemKwWith
	ItemKwRec
)

type Item struct {
	Type ItemType
	Value any
}

func (i Item) AsString() string {
	if s, ok := i.Value.(string); ok {
		return s
	}
	return ""
}

func (i Item) AsInt() int64 {
	if s, ok := i.Value.(int64); ok {
		return s
	}
	return 0
}

func (i Item) AsFloat() float64 {
	if s, ok := i.Value.(float64); ok {
		return s
	}
	return 0.0
}

func (i Item) AsError() error {
	if s, ok := i.Value.(error); ok {
		return s
	}
	return errors.New("???")
}

func (i Item) String() string {
	switch i.Type {
	case ItemError:
		return "lexer error: " + i.AsError().Error()
	case ItemEOF:
		return "EOF!"
	case ItemPlus:
		return "+"
	case ItemMinus:
		return "-"
	case ItemMul:
		return "*"
	case ItemDiv:
		return "/"
	case ItemEquals:
		return "="
	case ItemEqualsEquals:
		return "=="
	case ItemExclamation:
		return "!"
	case ItemExclamationEquals:
		return "!="
	case ItemQuestion:
		return "?"
	case ItemDot:
		return "."
	case ItemComma:
		return ","
	case ItemLt:
		return "<"
	case ItemGt:
		return ">"
	case ItemAt:
		return "@"
	case ItemSemicolon:
		return ";"
	case ItemColon:
		return ":"
	case ItemLeftParen:
		return "("
	case ItemRightParen:
		return ")"
	case ItemLeftBrace:
		return "{"
	case ItemRightBrace:
		return "}"
	case ItemLeftBracket:
		return "["
	case ItemRightBracket:
		return "]"
	case ItemInt:
		return strconv.FormatInt(i.AsInt(), 10)
	case ItemFloat:
		return strconv.FormatFloat(i.AsFloat(), 'f', -1, 64)
	case ItemString:
		return "\"" + i.AsString() + "\""
	case ItemPathString:
		return "'" + i.AsString() + "'"
	case ItemIdentifier:
		return i.AsString()
	case ItemKwTrue:
		return "true"
	case ItemKwFalse:
		return "false"
	case ItemKwNull:
		return "null"
	case ItemKwIf:
		return "if"
	case ItemKwThen:
		return "then"
	case ItemKwElse:
		return "else"
	case ItemKwInherit:
		return "inherit"
	case ItemKwWith:
		return "with"
	case ItemKwRec:
		return "rec"
	default:
		return "???"
	}
}

type Lexer struct {
	input string
	pos int // offset in string
	width int // width of current rune
	futureToken Item // next token
}

func New(input string) *Lexer {
	lex := Lexer{input: input, pos: 0}
	lex.Next() // to initialize futureToken so the first Peek or Next will return something
	return &lex
}

func (lex *Lexer) Peek() Item {
	return lex.futureToken
}

func (lex *Lexer) Next() Item {
	current := lex.futureToken
	lex.futureToken = lex.nextToken()
	return current
}

func (lex *Lexer) nextToken() Item {
	err := lex.skipCommentsAndWhitespace()
	if err != nil {
		if errors.Is(err, io.EOF) {
			return Item{Type: ItemEOF, Value: nil}
		}
		return Item{Type: ItemError, Value: err}
	}
	r, err := lex.next()
	if err != nil {
		if errors.Is(err, io.EOF) {
			return Item{Type: ItemEOF, Value: nil}
		}
		return Item{Type: ItemError, Value: err}
	}

	// single character stuff
	switch r {
	case '+':
		return Item{Type: ItemPlus, Value: nil}
	case '-':
		return Item{Type: ItemMinus, Value: nil}
	case '*':
		return Item{Type: ItemMul, Value: nil}
	case '/':
		return Item{Type: ItemDiv, Value: nil}
	case '?':
		return Item{Type: ItemQuestion, Value: nil}
	case '.':
		return Item{Type: ItemDot, Value: nil}
	case ',':
		return Item{Type: ItemComma, Value: nil}
	case '@':
		return Item{Type: ItemAt, Value: nil}
	case ';':
		return Item{Type: ItemSemicolon, Value: nil}
	case '<':
		return Item{Type: ItemLt, Value: nil}
	case '>':
		return Item{Type: ItemGt, Value: nil}
	case ':':
		return Item{Type: ItemColon, Value: nil}
	case '(':
		return Item{Type: ItemLeftParen, Value: nil}
	case ')':
		return Item{Type: ItemRightParen, Value: nil}
	case '{':
		return Item{Type: ItemLeftBrace, Value: nil}
	case '}':
		return Item{Type: ItemRightBrace, Value: nil}
	case '[':
		return Item{Type: ItemLeftBracket, Value: nil}
	case ']':
		return Item{Type: ItemRightBracket, Value: nil}
	}
	lex.back()

	// FIXME: this is cursed. The error handling is a damn mess everywhere :c
	// =, ==, ! and !=
	start := lex.pos
	r1, err := lex.next()
	if err != nil {
		if errors.Is(err, io.EOF) {
			return Item{Type: ItemEOF, Value: nil}
		}
		return Item{Type: ItemError, Value: err}
	}
	startr2 := lex.pos
	r2, err := lex.next()
	if err != nil {
		if errors.Is(err, io.EOF) {
			return Item{Type: ItemEOF, Value: nil}
		}
		return Item{Type: ItemError, Value: err}
	}
	if r1 == '=' {
		if r2 == '=' {
			return Item{Type: ItemEqualsEquals, Value: nil}
		}
		lex.pos = startr2
		return Item{Type: ItemEquals, Value: nil}
	}
	if r1 == '!' {
		if r2 == '=' {
			return Item{Type: ItemExclamationEquals, Value: nil}
		}
		lex.pos = startr2
		return Item{Type: ItemExclamation, Value: nil}
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

	return Item{Type: ItemError, Value: errors.New("unexpected token: " + string(r))}
}

func (lex *Lexer) scanNumber() Item {
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
			return Item{Type: ItemError, Value: err}
		}
		// float
		if r == '.' {
			// extra dot or hex? Impossible.
			if isFloat {
				return Item{Type: ItemError, Value: errors.New("more than one dot float")}
			}
			if isHex {
				return Item{Type: ItemError, Value: errors.New("hex & float number???")}
			}
			isFloat = true
			continue
		}
		// hex
		if r == 'x' {
			// needs to start with 0x for hex
			if !(lex.pos - start == 2 && lex.input[start] == '0') {
				return Item{Type: ItemError, Value: errors.New("invalid hex number")}
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
			return Item{Type: ItemError, Value: err}
		}
		return Item{Type: ItemFloat, Value: val}
	} else {
		base := 10
		offset := 0
		if isHex {
			base = 16
			offset = 2
		}
		val, err := strconv.ParseInt(lex.input[start+offset:start+n], base, 64)
		if err != nil {
			return Item{Type: ItemError, Value: err}
		}
		return Item{Type: ItemInt, Value: val}
	}
}

func (lex *Lexer) scanIdentifier() Item {
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
			return Item{Type: ItemError, Value: err}
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
		return Item{Type: ItemKwTrue, Value: nil}
	case "false":
		return Item{Type: ItemKwFalse, Value: nil}
	case "null":
		return Item{Type: ItemKwNull, Value: nil}
	case "if":
		return Item{Type: ItemKwIf, Value: nil}
	case "then":
		return Item{Type: ItemKwThen, Value: nil}
	case "else":
		return Item{Type: ItemKwElse, Value: nil}
	case "inherit":
		return Item{Type: ItemKwInherit, Value: nil}
	case "with":
		return Item{Type: ItemKwWith, Value: nil}
	case "rec":
		return Item{Type: ItemKwRec, Value: nil}
	}

	return Item{Type: ItemIdentifier, Value: ident}
}

func (lex *Lexer) scanString(isPath bool) Item {
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
				return Item{Type: ItemEOF, Value: nil}
			}
			return Item{Type: ItemError, Value: err}
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
		return Item{Type: ItemPathString, Value: result}
	} else {
		return Item{Type: ItemString, Value: result}
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
