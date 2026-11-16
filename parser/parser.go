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
	case lexer.TokenIdentifier:
		lhs = ast.Identifier{Value: token.AsString()}
	case lexer.TokenExclamation:
		expr, err := parse(lex, 50)
		if err != nil {
			return expr, err
		}
		lhs = ast.UnaryOp{Op: ast.UnaryOpBoolInvert, Rhs: expr}
	case lexer.TokenMinus:
		expr, err := parse(lex, 50)
		if err != nil {
			return expr, err
		}
		lhs = ast.UnaryOp{Op: ast.UnaryOpNegate, Rhs: expr}
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
		// HACK: this whole thing is peak hack :3c fuck clean code we ball this shit is unreadable!! Good luck fixing it future me :3
		mode := 0 // 0 = to be detected, 1 = attrset 2 = attrset function argument
		isRec := token.Type == lexer.TokenKwRec
		if isRec {
			mode = 1 // rec = must be an attrset!
			nextToken := lex.Next()
			if nextToken.Type != lexer.TokenLeftBrace {
				return lhs, errors.New("expected { got: " + nextToken.String())
			}
		}
		// stuff for attrset
		attrsItems := []ast.AttrsetItem{}
		// stuff for attrset function argument
		attrsLambdaArgs := []ast.AttrsetLambdaArgItem{}
		ignoreExtraAttrs := false
		attrsLoop:
		for {
			nextToken := lex.Peek()
			if nextToken.Type == lexer.TokenRightBrace {
				lex.Next()
				break
			}
			var exprNamePtr *ast.Expr = nil
			switch mode {
			case 0: // detect
				switch nextToken.Type {
				case lexer.TokenTripleDot:
					mode = 2
				case lexer.TokenIdentifier:
					// faaaah now we got a pain in the ass cuz we need to know the token after this, so lets parse the name and see
					exprName, err := parse(lex, 0)
					if err != nil {
						return exprName, err
					}
					exprNamePtr = &exprName
					nextToken = lex.Peek()
					if nextToken.Type == lexer.TokenComma {
						mode = 2
						goto case0Fallthrough // HACK: The attrset case will fallthrough again if mode == 2 which then leads us to the correct case :DDDD
					} else {
						// this is an attrset
						mode = 1
						goto case0Fallthrough
					}
				default:
					// this is an attrset
					mode = 1
				}
				continue
				case0Fallthrough:
			fallthrough
			case 1: // attrset
				var exprValue ast.Expr
				var err error
				if mode == 2 {
					goto case1Fallthrough
				}
				if exprNamePtr == nil {
					exprName, err := parse(lex, 0)
					if err != nil {
						return exprName, err
					}
					exprNamePtr = &exprName
				}
				nextToken = lex.Next()
				if nextToken.Type != lexer.TokenEquals {
					return lhs, errors.New("expected = got: " + nextToken.String())
				}
				exprValue, err = parse(lex, 0)
				if err != nil {
					return exprValue, err
				}
				nextToken = lex.Next()
				if nextToken.Type != lexer.TokenSemicolon {
					return lhs, errors.New("expected ; got: " + nextToken.String())
				}
				attrsItems = append(attrsItems, ast.AttrsetItem{Name: *exprNamePtr, Value: exprValue})
				continue
				case1Fallthrough:
			fallthrough
			case 2: // attrset function argument
				var nextToken lexer.Token
				// special case in case we fell through from detect
				if exprNamePtr != nil {
					identExpr, ok := (*exprNamePtr).(ast.Identifier)
					if !ok {
						return lhs, errors.New("expected ast.Identifier")
					}
					nextToken = lexer.Token{Type: lexer.TokenIdentifier, Value: identExpr.Value}
				} else {
					nextToken = lex.Next()
				}
				switch nextToken.Type {
				case lexer.TokenIdentifier:
				case lexer.TokenTripleDot:
					ignoreExtraAttrs = true
					// check the next token to make sure it's } since ... should always be the last thing
					nextToken = lex.Next()
					if nextToken.Type != lexer.TokenRightBrace {
						return lhs, errors.New("expected } after ... got: " + nextToken.String())
					}
					break attrsLoop
				default:
					return lhs, errors.New("expected identifier or ... got: " + nextToken.String())
				}
				// if we are here nextToken is an identifier
				name := nextToken.AsString()

				hasDefault := false
				var exprDefault ast.Expr
				nextToken = lex.Peek()
				if nextToken.Type == lexer.TokenQuestion {
					lex.Next() // consume question mark
					hasDefault = true
					var err error
					exprDefault, err = parse(lex, 0)
					if err != nil {
						return exprDefault, err
					}
					// peek the next token again for the code below
					nextToken = lex.Peek()
				}
				switch nextToken.Type {
				case lexer.TokenComma:
					lex.Next() // consume comma
				case lexer.TokenRightBrace:
				default:
					return lhs, errors.New("expected , or } got: " + nextToken.String())
				}
				attrsLambdaArgs = append(attrsLambdaArgs, ast.AttrsetLambdaArgItem{Name: name, HasDefault: hasDefault, Default: exprDefault})
				continue
			}
		}
		switch mode {
		case 0, 1: // attrset
			lhs = ast.Attrset{Items: attrsItems, Recursive: isRec}
		case 2:
			hasBind := false
			var bind string
			nextToken := lex.Peek()
			if nextToken.Type == lexer.TokenAt {
				lex.Next() // consume @
				nextToken := lex.Next()
				if nextToken.Type != lexer.TokenIdentifier {
					return lhs, errors.New("expected identifier after @ got: " + nextToken.String())
				}
				hasBind = true
				bind = nextToken.AsString()
			}
			lhs = ast.AttrsetLambdaArg{Items: attrsLambdaArgs, IgnoreExtraAttrs: ignoreExtraAttrs, HasBind: hasBind, Bind: bind}
		}
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

		// Lambda
		if nextToken.Type == lexer.TokenColon {
			switch lhs.(type) {
			case ast.Identifier, ast.AttrsetLambdaArg:
			default:
				return lhs, errors.New("invalid function arg type")
			}

			// consume colon
			lex.Next()

			exprBody, err := parse(lex, 0)
			if err != nil {
				return exprBody, err
			}
			lhs = ast.Lambda{Arg: lhs, Body: exprBody}
			continue
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
		return 6, 6
	case ast.BinOpMul, ast.BinOpDiv:
		return 7, 8
	case ast.BinOpLt, ast.BinOpGt:
		return 3, 4
	case ast.BinOpEq, ast.BinOpNeq:
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
	case lexer.TokenEqualsEquals:
		op = ast.BinOpEq
	case lexer.TokenExclamationEquals:
		op = ast.BinOpNeq
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
