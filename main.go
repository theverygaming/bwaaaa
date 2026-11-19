package main

import "os"
import "log"
import "fmt"
import "bwaaaa/bwaaaa/lexer"
import "bwaaaa/bwaaaa/parser"
import "bwaaaa/bwaaaa/interpreter"
import "path/filepath"

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
    importedFiles := make(map[string]any)
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
            "import": interpreter.NewThunkEvaluated(interpreter.NewFnDefExternal(func(ip *interpreter.Interpreter, argT *interpreter.Thunk) (any, error) {
                arg, err := ip.Force(argT)
                if err != nil {
                    return nil, err
                }
                if argStr, isString := arg.(string); isString {
                    // prepare path
                    abspath, err := filepath.Abs(argStr)
                    if err != nil {
                        return nil, err
                    }
                    // check if already imported
                    val, exists := importedFiles[abspath]
                    if exists {
                        return val, nil
                    }
                    // parse
                    content, err := os.ReadFile(abspath)
                    if err != nil {
                        return nil, err
                    }
                    expr, err := parser.Parse(lexer.New(string(content)))
                    if err != nil {
                        return nil, err
                    }
                    // eval & save to list of imported files
                    val, err = ip.EvalLazy(expr)
                    if err != nil {
                        return nil, err
                    }
                    importedFiles[abspath] = val
                    return val, nil
                } else {
                    return nil, fmt.Errorf("import called with non-string argument %T: %v", arg, arg)
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
