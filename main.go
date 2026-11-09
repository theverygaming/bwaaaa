package main

import "os"
import "log"
import "fmt"
import "bwaaaa/bwaaaa/lexer"

func main() {
    content, err := os.ReadFile("samples/lang.bwaaaa");
    if err != nil {
        log.Fatal(err)
    }
    lex := lexer.New(string(content))
    for {
        token := lex.NextToken()
        fmt.Println(token)
        if token.Type == lexer.ItemError || token.Type == lexer.ItemEOF {
            break
        }
    }
}
