// big thanks to: https://matklad.github.io/2020/04/13/simple-but-powerful-pratt-parsing.html
package parser

import "bwaaaa/bwaaaa/lexer"
import "bwaaaa/bwaaaa/ast"
import "errors"

func Parse(lex *lexer.Lexer) (ast.Expr, error) {
	expr, err := parse(lex, 0)
	if err == nil {
		token := lex.Next()
		if token.Type == lexer.TokenError {
			return expr, token.AsError()
		}
		if token.Type != lexer.TokenEOF {
			return expr, errors.New("expected EOF got: " + token.String())
		}
	}
	return expr, err
}

func parse(lex *lexer.Lexer, minBp int) (ast.Expr, error) {
	var lhs ast.Expr
	token := lex.Next()
	if token.Type == lexer.TokenError {
		return lhs, token.AsError()
	}

	switch token.Type {
	case lexer.TokenKwTrue:
		lhs = ast.Boolean{Value: true}
	case lexer.TokenKwFalse:
		lhs = ast.Boolean{Value: false}
	case lexer.TokenKwNull:
		lhs = ast.Null{}
	case lexer.TokenInt:
		lhs = ast.Integer{Value: token.AsInt()}
	case lexer.TokenFloat:
		lhs = ast.Float{Value: token.AsFloat()}
	case lexer.TokenString:
		lhs = ast.String{Value: token.AsString()}
	case lexer.TokenPathString:
		lhs = ast.PathString{Value: token.AsString()}
	case lexer.TokenIdentifier:
		lhs = ast.Identifier{Value: token.AsString()}
	// if..then..else
	case lexer.TokenKwIf:
		exprIf, err := parse(lex, 0)
		if err != nil {
			return exprIf, err
		}
		nextToken := lex.Next()
		if nextToken.Type != lexer.TokenKwThen {
			return lhs, errors.New("expected then got: " + nextToken.String())
		}
		exprThen, err := parse(lex, 0)
		if err != nil {
			return exprThen, err
		}
		nextToken = lex.Next()
		if nextToken.Type != lexer.TokenKwElse {
			return lhs, errors.New("expected else got: " + nextToken.String())
		}
		exprElse, err := parse(lex, 0)
		if err != nil {
			return exprElse, err
		}
		lhs = ast.IfThenElse{If: exprIf, Then: exprThen, Else: exprElse}
	// let..in
	case lexer.TokenKwLet:
		vars := []ast.LetInVar{}
		for {
			nextToken := lex.Next()
			if nextToken.Type == lexer.TokenKwIn {
				break
			}
			if nextToken.Type != lexer.TokenIdentifier {
				return lhs, errors.New("expected identifier got: " + nextToken.String())
			}
			varName := nextToken.AsString()
			nextToken = lex.Next()
			if nextToken.Type != lexer.TokenEquals {
				return lhs, errors.New("expected = got: " + nextToken.String())
			}
			exprValue, err := parse(lex, 0)
			if err != nil {
				return exprValue, err
			}
			nextToken = lex.Next()
			if nextToken.Type != lexer.TokenSemicolon {
				return lhs, errors.New("expected ; got: " + nextToken.String())
			}
			vars = append(vars, ast.LetInVar{Name: varName, Value: exprValue})
		}
		exprIn, err := parse(lex, 0)
		if err != nil {
			return exprIn, err
		}
		lhs = ast.LetIn{Vars: vars, In: exprIn}
	case lexer.TokenKwWith:
		exprWith, err := parse(lex, 0)
		if err != nil {
			return exprWith, err
		}
		nextToken := lex.Next()
		if nextToken.Type != lexer.TokenSemicolon {
			return lhs, errors.New("expected ; got: " + nextToken.String())
		}
		exprIn, err := parse(lex, 0)
		if err != nil {
			return exprIn, err
		}
		lhs = ast.WithIn{With: exprWith, In: exprIn}
	case lexer.TokenLeftBracket:
		items := []ast.Expr{}
		for {
			nextToken := lex.Peek()
			if nextToken.Type == lexer.TokenRightBracket {
				lex.Next()
				break
			}
			expr, err := parse(lex, 201) // minBp > the binding power for attribute selection to prevent stuff like [a.b c.d] from being valid (instead do [(a.b) (c.d)])
			if err != nil {
				return expr, err
			}
			items = append(items, expr)
		}
		lhs = ast.List{Items: items}
	case lexer.TokenKwRec:
		fallthrough
	case lexer.TokenLeftBrace:
		isRec := token.Type == lexer.TokenKwRec
		if isRec {
			nextToken := lex.Next()
			if nextToken.Type != lexer.TokenLeftBrace {
				return lhs, errors.New("expected { got: " + nextToken.String())
			}
		}
		items := []ast.AttrsetItem{}
		for {
			nextToken := lex.Peek()
			if nextToken.Type == lexer.TokenRightBrace {
				lex.Next()
				break
			}
			exprName, err := parse(lex, 0)
			if err != nil {
				return exprName, err
			}
			nextToken = lex.Next()
			if nextToken.Type != lexer.TokenEquals {
				return lhs, errors.New("expected = got: " + nextToken.String())
			}
			exprValue, err := parse(lex, 0)
			if err != nil {
				return exprValue, err
			}
			nextToken = lex.Next()
			if nextToken.Type != lexer.TokenSemicolon {
				return lhs, errors.New("expected ; got: " + nextToken.String())
			}
			items = append(items, ast.AttrsetItem{Name: exprName, Value: exprValue})
		}
		lhs = ast.Attrset{Items: items, Recursive: isRec}
	case lexer.TokenLeftParen:
		var err error
		lhs, err = parse(lex, 0)
		if err != nil {
			return lhs, err
		}
		nextToken := lex.Next()
		if nextToken.Type != lexer.TokenRightParen {
			return lhs, errors.New("expected ) got: " + nextToken.String())
		}
	// TODO: prefix operators (-, !)
	default:
		return lhs, errors.New("unexpected token: " + token.String())
	}

	for {
		nextToken := lex.Peek()
		if nextToken.Type == lexer.TokenError {
			return lhs, nextToken.AsError()
		}
		if nextToken.Type == lexer.TokenEOF {
			break
		}

		// Attribute Selection
		if nextToken.Type == lexer.TokenDot {
			lhsBp := 200
			if lhsBp < minBp {
				break
			}

			// consume dot
			lex.Next()

			var attr string
			attrToken := lex.Next()
			switch attrToken.Type {
			case lexer.TokenIdentifier, lexer.TokenString:
				attr = attrToken.AsString()
			default:
				return lhs, errors.New("Attribute Selection - expected Identifier or String, got: " + attrToken.String())
			}
			lhs = ast.AttrGet{Lhs: lhs, Attr: attr}
			continue
		}

		// Infix
		op, isInfix := infixOpConv(nextToken.Type)
		if isInfix {
			lhsBp, rhsBp := infixBindingPower(op) 
			if lhsBp < minBp {
				break
			}
			lex.Next()

			rhs, err := parse(lex, rhsBp)
			if err != nil {
				return rhs, err
			}
			lhs = ast.BinOp{Op: op, Lhs: lhs, Rhs: rhs}
			continue
		}

		if isValidFunctionArg(nextToken.Type) {
			lhsBp, rhsBp := 100, 101
			if lhsBp < minBp {
				break
			}
			rhs, err := parse(lex, rhsBp)
			if err != nil {
				return rhs, err
			}
			lhs = ast.FnCall{Fn: lhs, Arg: rhs}
			continue
		}

		break
	}

	return lhs, nil
}

// returns: lhsBp, rhsBp
func infixBindingPower(t ast.BinOpType) (int, int) {
	switch t {
	case ast.BinOpAdd, ast.BinOpSub:
		return 3, 4
	case ast.BinOpMul, ast.BinOpDiv:
		return 5, 6
	case ast.BinOpLt, ast.BinOpGt:
		return 1, 2
	default:
		return 0, 0
	}
}

func infixOpConv(t lexer.TokenType) (ast.BinOpType, bool) {
	var op ast.BinOpType
	isInfix := true
	switch t {
	case lexer.TokenPlus:
		op = ast.BinOpAdd
	case lexer.TokenMinus:
		op = ast.BinOpSub
	case lexer.TokenMul:
		op = ast.BinOpMul
	case lexer.TokenDiv:
		op = ast.BinOpDiv
	case lexer.TokenLt:
		op = ast.BinOpLt
	case lexer.TokenGt:
		op = ast.BinOpGt
	default:
		isInfix = false
	}
	return op, isInfix
}

func isValidFunctionArg(t lexer.TokenType) bool {
	switch t {
	case lexer.TokenExclamation,
		lexer.TokenMinus,
		lexer.TokenLeftParen,
		lexer.TokenLeftBrace,
		lexer.TokenLeftBracket,
		lexer.TokenInt,
		lexer.TokenFloat,
		lexer.TokenString,
		lexer.TokenPathString,
		lexer.TokenIdentifier,
		lexer.TokenKwTrue,
		lexer.TokenKwFalse,
		lexer.TokenKwNull,
		lexer.TokenKwRec:
		return true
	default:
		return false
	}
}
