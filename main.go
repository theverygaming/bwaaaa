package main

import "os"
import "log"
import "fmt"
import "bwaaaa/bwaaaa/lexer"
import "bwaaaa/bwaaaa/parser"
import "bwaaaa/bwaaaa/interpreter"

func main() {
    content, err := os.ReadFile("samples/lazy-eval.bwaaaa")
    if err != nil {
        log.Fatal(err)
    }
    lex := lexer.New(string(content))
    expr, err := parser.Parse(lex)
    if err != nil {
        log.Fatal(err)
    }
    //fmt.Println(expr.Dump(0, 2))
    env := map[string]*interpreter.Thunk{
        "builtins": interpreter.NewThunkEvaluated(map[string]*interpreter.Thunk{
            "throw": interpreter.NewThunkEvaluated(interpreter.NewFnDefExternal(func(ip *interpreter.Interpreter, argT *interpreter.Thunk) (any, error) {
                arg, err := ip.Force(argT)
                if err != nil {
                    return nil, err
                }
                if argStr, isString := arg.(string); isString {
                    return nil, fmt.Errorf("throw called! message: '%s'", argStr)
                } else {
                    return nil, fmt.Errorf("throw called! with unknown arg of type %T: %v", arg, arg)
                }
            })),
        }),
    }
    ip := interpreter.New(env)
    val, err := ip.Eval(expr)
    if err != nil {
        log.Fatal(err)
    }
    fmt.Println(val)
}
