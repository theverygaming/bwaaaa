package main

import "os"
import "log"
import "fmt"
import "bwaaaa/bwaaaa/lexer"
import "bwaaaa/bwaaaa/parser"

func main() {
    content, err := os.ReadFile("samples/lang.bwaaaa");
    if err != nil {
        log.Fatal(err)
    }
    lex := lexer.New(string(content))
    expr, err := parser.Parse(lex)
    if err != nil {
        log.Fatal(err)
    }
    fmt.Println(expr.Dump(0, 2))
    for {
        token := lex.Next()
        fmt.Println(token)
        if token.Type == lexer.ItemError || token.Type == lexer.ItemEOF {
            break
        }
    }
}
