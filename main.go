package main

import "os"
import "log"
import "fmt"
import "bwaaaa/bwaaaa/lexer"
import "bwaaaa/bwaaaa/parser"
import "bwaaaa/bwaaaa/interpreter"
import "path/filepath"
import "strings"
import "encoding/base64"
import "crypto/sha256"
import "io"
import "net/http"

func main() {
    content, err := os.ReadFile("meow/meow.bwaaaa")
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
            "fetchhttp": interpreter.NewThunkEvaluated(interpreter.NewFnDefExternal(func(ip *interpreter.Interpreter, argT *interpreter.Thunk) (any, error) {
                arg, err := ip.Force(argT)
                if err != nil {
                    return nil, err
                }
                argAttrs, isAttrs := arg.(map[string]*interpreter.Thunk)
                if !isAttrs {
                    return nil, fmt.Errorf("fetchhttp called with unknown arg of type %T: %v", arg, arg)
                }

                // get name arg
                name := "fetchhttp"
                nameT := argAttrs["name"]
                if nameT != nil {
                    nameF, err := ip.Force(nameT)
                    if err != nil {
                        return nil, err
                    }
                    nameS, isString := nameF.(string)
                    if !isString {
                        return nil, fmt.Errorf("fetchhttp called with invalid type for name arg %T: %v", nameF, nameF)
                    }
                    name = nameS
                }

                // get url arg
                urlT := argAttrs["url"]
                if urlT == nil {
                    return nil, fmt.Errorf("fetchhttp called with missing argument url")
                }
                urlF, err := ip.Force(urlT)
                if err != nil {
                    return nil, err
                }
                url, isString := urlF.(string)
                if !isString {
                    return nil, fmt.Errorf("fetchhttp called with invalid type for url arg %T: %v", urlF, urlF)
                }

                // get hash arg
                hashT := argAttrs["hash"]
                if hashT == nil {
                    return nil, fmt.Errorf("fetchhttp called with missing argument hash")
                }
                hashF, err := ip.Force(hashT)
                if err != nil {
                    return nil, err
                }
                hash, isString := hashF.(string)
                if !isString {
                    return nil, fmt.Errorf("fetchhttp called with invalid type for hash arg %T: %v", hashF, hashF)
                }

                // get executable arg
                executable := false
                executableT := argAttrs["executable"]
                if executableT != nil {
                    executableF, err := ip.Force(executableT)
                    if err != nil {
                        return nil, err
                    }
                    executableB, isString := executableF.(bool)
                    if !isString {
                        return nil, fmt.Errorf("fetchhttp called with invalid type for executable arg %T: %v", executableF, executableF)
                    }
                    executable = executableB
                }

                // parse the hash
                if hash == "" {
                    hash = "sha256-AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA="
                    fmt.Println("fetchhttp: empty hash encountered. Assuming", hash)
                }
                if !strings.HasPrefix(hash, "sha256-") {
                    return nil, fmt.Errorf("fetchhttp called with unsupported hash format %s", hash)
                }

                // HTTP request
                resp, err := http.Get(url)
                if err != nil {
                    return nil, err
                }
                defer resp.Body.Close()
                if resp.StatusCode >= 300 {
                    return nil, fmt.Errorf("unexpected HTTP status fetching URL: %s code: %d", url, resp.StatusCode)
                }

                // prepare a temporary file
                tmpf, err := os.CreateTemp("", name)
                if err != nil {
                    return nil, err
                }
                defer os.Remove(tmpf.Name())

                // write to the temporary file & hash
                hasher := sha256.New()
                mwriter := io.MultiWriter(hasher, tmpf)
                _, err = io.Copy(mwriter, resp.Body)
                if err != nil {
                    tmpf.Close()
                    return nil, err
                }
                if err := tmpf.Close(); err != nil {
                    return nil, err
                }

                // verify the hash
                hashGot := "sha256-" + base64.StdEncoding.EncodeToString(hasher.Sum(nil))
                if hashGot != hash {
                    return nil, fmt.Errorf("fetchhttp expected hash %s got: %s", hash, hashGot)
                }

                // set the correct permissions on the temporary file
                var destPerms os.FileMode = 0444
                if executable {
                    destPerms = 0555
                }
                if err := os.Chmod(tmpf.Name(), destPerms); err != nil {
                    return nil, err
                }

                // TODO: we need a store implementation then we could actually copy the temporary file to the store :)
                outPath := tmpf.Name() + "-fetched"
                if err := os.Rename(tmpf.Name(), outPath); err != nil {
                    return nil, err
                }
                return outPath, nil
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
