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
    ip := interpreter.New()
    val, err := ip.Eval(expr)
    if err != nil {
        log.Fatal(err)
    }
    fmt.Println(val)
}
