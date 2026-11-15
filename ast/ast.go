package ast

import "strconv"
import "strings"

type Expr interface {
	Dump(indent int, indentIncrement int) string
}

type BinOpType int

const (
	BinOpAdd BinOpType = iota
	BinOpSub
	BinOpMul
	BinOpDiv
	BinOpLt
	BinOpGt
)

type BinOp struct {
	Op BinOpType
	Lhs Expr
	Rhs Expr
}

func indentString(indent int) string {
	return strings.Repeat(" ", indent)
}

func (x BinOp) Dump(indent int, indentIncrement int) string {
	var op string
	switch x.Op {
	case BinOpAdd:
		op = "+"
	case BinOpSub:
		op = "-"
	case BinOpMul:
		op = "*"
	case BinOpDiv:
		op = "/"
	case BinOpLt:
		op = "<"
	case BinOpGt:
		op = ">"
	}
	ids := indentString(indent)
	idn := indent + indentIncrement
	return ids + "(BinOp " + op + "\n" +
			x.Lhs.Dump(idn, indentIncrement) + "\n" + 
			x.Rhs.Dump(idn, indentIncrement) + "\n" +
			ids + ")"
}

type FnCall struct {
	Fn Expr
	Arg Expr
}

func (x FnCall) Dump(indent int, indentIncrement int) string {
	ids := indentString(indent)
	idn := indent + indentIncrement
	return ids + "(FnCall\n" +
			x.Fn.Dump(idn, indentIncrement) + "\n" + 
			x.Arg.Dump(idn, indentIncrement) + "\n" +
			ids + ")"
}

type AttrGet struct {
	Lhs Expr
	Attr string
}

func (x AttrGet) Dump(indent int, indentIncrement int) string {
	ids := indentString(indent)
	idn := indent + indentIncrement
	return ids + "(AttrGet\n" +
			x.Lhs.Dump(idn, indentIncrement) + "\n" + 
			indentString(idn) + "(Attr " + x.Attr + ")\n" +
			ids + ")"
}

type IfThenElse struct {
	If Expr
	Then Expr
	Else Expr
}

func (x IfThenElse) Dump(indent int, indentIncrement int) string {
	ids := indentString(indent)
	idn := indent + indentIncrement
	return ids + "(IfThenElse\n" +
			x.If.Dump(idn, indentIncrement) + "\n" + 
			x.Then.Dump(idn, indentIncrement) + "\n" +
			x.Else.Dump(idn, indentIncrement) + "\n" +
			ids + ")"
}

type LetInVar struct {
	Name string
	Value Expr
}

type LetIn struct {
	Vars []LetInVar
	In Expr
}

func (x LetIn) Dump(indent int, indentIncrement int) string {
	ids := indentString(indent)
	idn := indent + indentIncrement
	vars := ""
	for _, v := range x.Vars {
		vars += indentString(idn) + "(Var " + v.Name + "\n" +
				v.Value.Dump(idn + indentIncrement, indentIncrement) + "\n" +
				indentString(idn) + ")\n"
	}
	return ids + "(LetIn\n" +
			vars +
			x.In.Dump(idn, indentIncrement) + "\n" +
			ids + ")"
}

type WithIn struct {
	With Expr
	In Expr
}

func (x WithIn) Dump(indent int, indentIncrement int) string {
	ids := indentString(indent)
	idn := indent + indentIncrement
	return ids + "(WithIn\n" +
			x.With.Dump(idn, indentIncrement) + "\n" + 
			x.In.Dump(idn, indentIncrement) + "\n" +
			ids + ")"
}

type List struct {
	Items []Expr
}

func (x List) Dump(indent int, indentIncrement int) string {
	ids := indentString(indent)
	idn := indent + indentIncrement
	items := ""
	for _, v := range x.Items {
		items += v.Dump(idn, indentIncrement) + "\n"
	}
	return ids + "(List\n" +
			items +
			ids + ")"
}

// basic types
type String struct {
	Value string
}

func (x String) Dump(indent int, indentIncrement int) string {
	return indentString(indent) + "(String \"" + x.Value + "\")"
}

type PathString struct {
	Value string
}

func (x PathString) Dump(indent int, indentIncrement int) string {
	return indentString(indent) + "(PathString '" + x.Value + "')"
}

type Identifier struct {
	Value string
}

func (x Identifier) Dump(indent int, indentIncrement int) string {
	return indentString(indent) + "(Identifier '" + x.Value + "')"
}

type Integer struct {
	Value int64
}

func (x Integer) Dump(indent int, indentIncrement int) string {
	return indentString(indent) + "(Integer " + strconv.FormatInt(x.Value, 10) + ")"
}

type Float struct {
	Value float64
}

func (x Float) Dump(indent int, indentIncrement int) string {
	return indentString(indent) + "(Float " + strconv.FormatFloat(x.Value, 'f', -1, 64) + ")"
}
