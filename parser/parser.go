// big thanks to: https://matklad.github.io/2020/04/13/simple-but-powerful-pratt-parsing.html
package parser

import "bwaaaa/bwaaaa/lexer"
import "bwaaaa/bwaaaa/ast"
import "errors"

func Parse(lex *lexer.Lexer) (ast.Expr, error) {
	return parse(lex, 0)
}

func parse(lex *lexer.Lexer, minBp int) (ast.Expr, error) {
	var lhs ast.Expr
	token := lex.Next()
	if token.Type == lexer.ItemError {
		return lhs, token.AsError()
	}

	switch token.Type {
	case lexer.ItemInt:
		lhs = ast.Integer{Value: token.AsInt()}
	case lexer.ItemFloat:
		lhs = ast.Float{Value: token.AsFloat()}
	case lexer.ItemString:
		lhs = ast.String{Value: token.AsString()}
	case lexer.ItemPathString:
		lhs = ast.PathString{Value: token.AsString()}
	case lexer.ItemLeftParen:
		var err error
		lhs, err = parse(lex, 0)
		if err != nil {
			return lhs, err
		}
		nextToken := lex.Next()
		if nextToken.Type != lexer.ItemRightParen {
			return lhs, errors.New("expected ) got: " + nextToken.String())
		}
	default:
		return lhs, errors.New("unknown token: " + token.String())
	}

	for {
		nextToken := lex.Peek()
		if nextToken.Type == lexer.ItemError {
			return lhs, nextToken.AsError()
		}
		if nextToken.Type == lexer.ItemEOF {
			break
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

func infixOpConv(t lexer.ItemType) (ast.BinOpType, bool) {
	var op ast.BinOpType
	isInfix := true
	switch (t) {
	case lexer.ItemPlus:
		op = ast.BinOpAdd
	case lexer.ItemMinus:
		op = ast.BinOpSub
	case lexer.ItemMul:
		op = ast.BinOpMul
	case lexer.ItemDiv:
		op = ast.BinOpDiv
	case lexer.ItemLt:
		op = ast.BinOpLt
	case lexer.ItemGt:
		op = ast.BinOpGt
	default:
		isInfix = false
	}
	return op, isInfix
}
