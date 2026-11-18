package interpreter

import "bwaaaa/bwaaaa/ast"
import "errors"
import "fmt"

type Interpreter struct {
	env *Env
}

func New() *Interpreter {
	ip := Interpreter{env: &Env{parent: nil, vars: make(map[string]*Thunk)}}
	return &ip
}

type Thunk struct {
	expr ast.Expr
	env *Env
	evaluated bool
	value any
}

type Env struct {
	parent *Env
	vars map[string]*Thunk
}

func NewEnv(parent *Env) *Env {
	return &Env{parent: parent, vars: make(map[string]*Thunk)}
}

func (env *Env) Get(key string) (*Thunk, error) {
	for env != nil {
		if (*env).vars[key] != nil {
			return (*env).vars[key], nil
		}
		env = (*env).parent
	}
	return nil, errors.New("undefined variable: " + key)
}

func (env *Env) Set(key string, thunk *Thunk) error {
	_, exists := (*env).vars[key]
	if exists {
		return errors.New("redefined variable: " + key)
	}
	(*env).vars[key] = thunk
	return nil
}

type FnDef struct {
	argName string
	env *Env
	body ast.Expr
}

func (ip *Interpreter) Eval(expr ast.Expr) (any, error) {
	return ip.evalForce(expr, ip.env)
}

func (ip *Interpreter) eval(expr ast.Expr, env *Env) (any, error) {
	switch node := expr.(type) {
	case ast.LetIn:
		newEnv := NewEnv(env)
		for _, v := range node.Vars {
			newEnv.Set(v.Name, &Thunk{
				expr: v.Value,
				env: newEnv,
				evaluated: false,
				value: nil,
			})
		}
		return ip.eval(node.In, newEnv)
	case ast.Identifier:
		return env.Get(node.Value)
	case ast.String:
		return node.Value, nil
	case ast.Attrset:
		attrsEnv := env
		if node.Recursive {
			attrsEnv = NewEnv(env)
		}
		items := make(map[string]*Thunk)
		for _, it := range node.Items {
			var name string
			if identifier, isIdentifier := it.Name.(ast.Identifier); isIdentifier {
				name = identifier.Value
			} else {
				val, err := ip.evalForce(it.Name, env)
				if err != nil {
					return val, err
				}
				if got, isString := val.(string); !isString {
					return nil, fmt.Errorf("attrset name expected string, got %T", val)
				} else {
					name = got
				}
			}

			valThunk := &Thunk{
				expr: it.Value,
				env: attrsEnv,
				evaluated: false,
				value: nil,
			}

			if node.Recursive {
				attrsEnv.Set(name, valThunk)
			}

			if items[name] != nil {
				return nil, fmt.Errorf("attrset key %s repeated", name)
			}
			items[name] = valThunk
		}
		return items, nil
	case ast.AttrGet:
		lhs, err := ip.evalForce(node.Lhs, env)
		if err != nil {
			return lhs, err
		}
		attrset, isAttrset := lhs.(map[string]*Thunk)
		if !isAttrset {
			return attrset, fmt.Errorf("expected attrset for AttrGet lhs, got %T", lhs)
		}
		val := attrset[node.Attr]
		if val == nil {
			return attrset, fmt.Errorf("attribute %s not in attrset", node.Attr)
		}
		return val, nil
	case ast.FnCall:
		// prepare call
		fn, err := ip.evalForce(node.Fn, env)
		if err != nil {
			return fn, err
		}
		fnDef, isFnDef := fn.(FnDef)
		if !isFnDef {
			return fnDef, fmt.Errorf("expected FnDef for FnCall function, got %T", fn)
		}
		// actually call
		callEnv := NewEnv(fnDef.env)
		callEnv.Set(fnDef.argName, &Thunk{
			expr: node.Arg,
			env: env,
			evaluated: false,
			value: nil,
		})
		res, err := ip.eval(fnDef.body, callEnv)
		if err != nil {
			return res, err
		}
		return res, nil
	case ast.Lambda:
		arg, isIdent := node.Arg.(ast.Identifier)
		if !isIdent {
			return arg, fmt.Errorf("expected Identifier for lambda arg, got %T", node.Arg)
		}
		return FnDef{
			argName: arg.Value,
			env: env,
			body: node.Body,
		}, nil
	case ast.BinOp:
		lhs, err := ip.evalForce(node.Lhs, env)
		if err != nil {
			return lhs, err
		}

		rhs, err := ip.evalForce(node.Rhs, env)
		if err != nil {
			return rhs, err
		}

		// FIXME: draw the rest of the fucking owl
		switch node.Op {
		case ast.BinOpAdd:
			if lhsStr, isLhsStr := lhs.(string); isLhsStr {
				if rhsStr, isRhsStr := rhs.(string); isRhsStr {
					return lhsStr + rhsStr, nil
				}
			}
		case ast.BinOpSub:
		case ast.BinOpMul:
		case ast.BinOpDiv:
		case ast.BinOpLt:
		case ast.BinOpGt:
		case ast.BinOpEq:
		case ast.BinOpNeq:
		}
		return nil, fmt.Errorf("Could not perform BinOp with %T and %T", lhs, rhs)
	default:
		return nil, errors.New("unknown node: " + node.Dump(0, 2))
	}
}

func (ip *Interpreter) evalForce(expr ast.Expr, env *Env) (any, error) {
	val, err := ip.eval(expr, env)
	if err != nil {
		return val, err
	}
	val, err = ip.force(val)
	if err != nil {
		return val, err
	}
	return val, nil
}

func (ip *Interpreter) force(val any) (any, error) {
	thunk, isThunk := val.(*Thunk)
	if !isThunk {
		return val, nil
	}

	if !thunk.evaluated {
		val, err := ip.eval(thunk.expr, thunk.env)
		if err != nil {
			return val, err
		}
		val, err = ip.force(val)
		if err != nil {
			return val, err
		}
		thunk.value = val
		thunk.evaluated = true
	}
	return thunk.value, nil
}
