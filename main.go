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
import "io"
import "net/http"
import "bwaaaa/bwaaaa/store"
import tarballlib "bwaaaa/bwaaaa/helpers/tarball"
import "bwaaaa/bwaaaa/derivation"

func main() {
    st, err := store.New("./meowstore/")
    if err != nil {
        log.Fatal(err)
    }
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

                // get tarball arg
                tarball := false
                tarballT := argAttrs["tarball"]
                if tarballT != nil {
                    tarballF, err := ip.Force(tarballT)
                    if err != nil {
                        return nil, err
                    }
                    tarballB, isString := tarballF.(bool)
                    if !isString {
                        return nil, fmt.Errorf("fetchhttp called with invalid type for tarball arg %T: %v", tarballF, tarballF)
                    }
                    tarball = tarballB
                }

                // some sanity checks
                if executable && tarball {
                    return nil, fmt.Errorf("fetchhttp: executable and tarball are mutually exclusive.")
                }

                // parse the hash
                if hash == "" {
                    hash = "sha256-AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA="
                    fmt.Println("fetchhttp: empty hash encountered. Assuming", hash)
                }
                if !strings.HasPrefix(hash, "sha256-") {
                    return nil, fmt.Errorf("fetchhttp called with unsupported hash format %s", hash)
                }
                ExpectedHashBytes, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(hash, "sha256-"))
                if err != nil {
                    return nil, err
                }

                // check if what we expect is already in the store
                _, err = st.GetPath(name, "default", ExpectedHashBytes)
                if err == nil {
                    return derivation.New(name, []string{"default"}, derivation.OutputFixed, ExpectedHashBytes, nil)
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

                var tmpPath string
                if !tarball {
                    // prepare a temporary file
                    tmpf, err := os.CreateTemp("", name)
                    if err != nil {
                        return nil, err
                    }
                    defer os.Remove(tmpf.Name())

                    // write to the temporary file
                    _, err = io.Copy(tmpf, resp.Body)
                    if err != nil {
                        tmpf.Close()
                        return nil, err
                    }
                    if err := tmpf.Close(); err != nil {
                        return nil, err
                    }

                    // set the correct permissions on the temporary file
                    var destPerms os.FileMode = 0444
                    if executable {
                        destPerms = 0555
                    }
                    if err := os.Chmod(tmpf.Name(), destPerms); err != nil {
                        return nil, err
                    }

                    tmpPath = tmpf.Name()
                } else {
                    // prepare a temporary directory
                    tmpd, err := os.MkdirTemp("", name)
                    if err != nil {
                        return nil, err
                    }
                    defer os.RemoveAll(tmpd)
                    if err := tarballlib.ExtractTarGz(tmpd, resp.Body); err != nil {
                        return nil, err
                    }
                    tmpPath = tmpd
                }

                // hash
                storeHash, err := store.Hash(tmpPath)
                if err != nil {
                    return nil, err
                }
                storeHashBytes := storeHash.Sum(nil)

                // verify the hash
                hashGot := "sha256-" + base64.StdEncoding.EncodeToString(storeHashBytes)
                if hashGot != hash {
                    return nil, fmt.Errorf("fetchhttp expected hash %s got: %s", hash, hashGot)
                }

                _, err = st.CopyToStore(tmpPath, name, "default", storeHashBytes)
                if err != nil {
                    return nil, err
                }

                return derivation.New(name, []string{"default"}, derivation.OutputFixed, storeHashBytes, nil)
            })),
            "derivation": interpreter.NewThunkEvaluated(interpreter.NewFnDefExternal(func(ip *interpreter.Interpreter, argT *interpreter.Thunk) (any, error) {
                arg, err := ip.Force(argT)
                if err != nil {
                    return nil, err
                }
                argAttrs, isAttrs := arg.(map[string]*interpreter.Thunk)
                if !isAttrs {
                    return nil, fmt.Errorf("derivation called with unknown arg of type %T: %v", arg, arg)
                }

                // get name arg
                nameT := argAttrs["name"]
                if nameT == nil {
                    return nil, fmt.Errorf("derivation called with missing argument name")
                }
                nameF, err := ip.Force(nameT)
                if err != nil {
                    return nil, err
                }
                name, isString := nameF.(string)
                if !isString {
                    return nil, fmt.Errorf("derivation called with invalid type for name arg %T: %v", nameF, nameF)
                }

                // get buildPlatform arg
                buildPlatformT := argAttrs["buildPlatform"]
                if buildPlatformT == nil {
                    return nil, fmt.Errorf("derivation called with missing argument buildPlatform")
                }
                buildPlatformF, err := ip.Force(buildPlatformT)
                if err != nil {
                    return nil, err
                }
                buildPlatform, isString := buildPlatformF.(string)
                if !isString {
                    return nil, fmt.Errorf("derivation called with invalid type for buildPlatform arg %T: %v", buildPlatformF, buildPlatformF)
                }

                // get builder arg
                builderT := argAttrs["builder"]
                if builderT == nil {
                    return nil, fmt.Errorf("derivation called with missing argument builder")
                }
                builderF, err := ip.Force(builderT)
                if err != nil {
                    return nil, err
                }
                builder, isString := builderF.(string)
                if !isString {
                    return nil, fmt.Errorf("derivation called with invalid type for builder arg %T: %v", builderF, builderF)
                }

                // get args arg
                args := []string{}
                argsT := argAttrs["args"]
                if argsT != nil {
                    argsF, err := ip.Force(argsT)
                    if err != nil {
                        return nil, err
                    }
                    argsL, isThunkList := argsF.([]*interpreter.Thunk)
                    if !isThunkList {
                        return nil, fmt.Errorf("derivation called with invalid type for args arg %T: %v", argsF, argsF)
                    }
                    for _, iT := range argsL {
                        iF, err := ip.Force(iT)
                        if err != nil {
                            return nil, err
                        }
                        iS, isString := iF.(string)
                        if !isString {
                            return nil, fmt.Errorf("derivation called with invalid type for args element %T: %v", iF, iF)
                        }
                        args = append(args, iS)
                    }
                }

                // get buildInputs arg
                buildInputs := []*derivation.Derivation{}
                buildInputsT := argAttrs["buildInputs"]
                if buildInputsT != nil {
                    buildInputsF, err := ip.Force(buildInputsT)
                    if err != nil {
                        return nil, err
                    }
                    buildInputsL, isThunkList := buildInputsF.([]*interpreter.Thunk)
                    if !isThunkList {
                        return nil, fmt.Errorf("derivation called with invalid type for buildInputs arg %T: %v", buildInputsF, buildInputsF)
                    }
                    for _, iT := range buildInputsL {
                        iF, err := ip.Force(iT)
                        if err != nil {
                            return nil, err
                        }
                        iS, isDerivation := iF.(*derivation.Derivation)
                        if !isDerivation {
                            return nil, fmt.Errorf("derivation called with invalid type for buildInputs element %T: %v", iF, iF)
                        }
                        buildInputs = append(buildInputs, iS)
                    }
                }

                // get runtimeInputs arg
                runtimeInputs := []*derivation.Derivation{}
                runtimeInputsT := argAttrs["runtimeInputs"]
                if runtimeInputsT != nil {
                    runtimeInputsF, err := ip.Force(runtimeInputsT)
                    if err != nil {
                        return nil, err
                    }
                    runtimeInputsL, isThunkList := runtimeInputsF.([]*interpreter.Thunk)
                    if !isThunkList {
                        return nil, fmt.Errorf("derivation called with invalid type for runtimeInputs arg %T: %v", runtimeInputsF, runtimeInputsF)
                    }
                    for _, iT := range runtimeInputsL {
                        iF, err := ip.Force(iT)
                        if err != nil {
                            return nil, err
                        }
                        iS, isDerivation := iF.(*derivation.Derivation)
                        if !isDerivation {
                            return nil, fmt.Errorf("derivation called with invalid type for runtimeInputs element %T: %v", iF, iF)
                        }
                        runtimeInputs = append(runtimeInputs, iS)
                    }
                }

                // get outputs arg
                outputs := []string{}
                outputsT := argAttrs["outputs"]
                if outputsT != nil {
                    outputsF, err := ip.Force(outputsT)
                    if err != nil {
                        return nil, err
                    }
                    outputsL, isThunkList := outputsF.([]*interpreter.Thunk)
                    if !isThunkList {
                        return nil, fmt.Errorf("derivation called with invalid type for outputs arg %T: %v", outputsF, outputsF)
                    }
                    for _, iT := range outputsL {
                        iF, err := ip.Force(iT)
                        if err != nil {
                            return nil, err
                        }
                        iS, isString := iF.(string)
                        if !isString {
                            return nil, fmt.Errorf("derivation called with invalid type for outputs element %T: %v", iF, iF)
                        }
                        outputs = append(outputs, iS)
                    }
                } else {
                    outputs = append(outputs, "default")
                }

                // get env arg
                env := make(map[string]string)
                envT := argAttrs["env"]
                if envT != nil {
                    envF, err := ip.Force(envT)
                    if err != nil {
                        return nil, err
                    }
                    envA, isThunkAttrs := envF.(map[string]*interpreter.Thunk)
                    if !isThunkAttrs {
                        return nil, fmt.Errorf("derivation called with invalid type for env arg %T: %v", envF, envF)
                    }
                    for key, iT := range envA {
                        iF, err := ip.Force(iT)
                        if err != nil {
                            return nil, err
                        }
                        iS, isString := iF.(string)
                        if !isString {
                            return nil, fmt.Errorf("derivation called with invalid type for env value %T: %v", iF, iF)
                        }
                        env[key] = iS
                    }
                }

                // get buildFiles arg
                buildFiles := make(map[string]string)
                buildFilesT := argAttrs["buildFiles"]
                if buildFilesT != nil {
                    buildFilesF, err := ip.Force(buildFilesT)
                    if err != nil {
                        return nil, err
                    }
                    buildFilesA, isThunkAttrs := buildFilesF.(map[string]*interpreter.Thunk)
                    if !isThunkAttrs {
                        return nil, fmt.Errorf("derivation called with invalid type for buildFiles arg %T: %v", buildFilesF, buildFilesF)
                    }
                    for key, iT := range buildFilesA {
                        iF, err := ip.Force(iT)
                        if err != nil {
                            return nil, err
                        }
                        iS, isString := iF.(string)
                        if !isString {
                            return nil, fmt.Errorf("derivation called with invalid type for buildFiles value %T: %v", iF, iF)
                        }
                        buildFiles[key] = iS
                    }
                }

                // get outputMode arg
                outputModeT := argAttrs["outputMode"]
                if outputModeT == nil {
                    return nil, fmt.Errorf("derivation called with missing argument outputMode")
                }
                outputModeF, err := ip.Force(outputModeT)
                if err != nil {
                    return nil, err
                }
                outputModeS, isString := outputModeF.(string)
                if !isString {
                    return nil, fmt.Errorf("derivation called with invalid type for outputMode arg %T: %v", outputModeF, outputModeF)
                }
                var outputMode derivation.DerivationOutputMode
                switch outputModeS {
                case "ia":
                    outputMode = derivation.OutputInputAddressed
                case "ca":
                    outputMode = derivation.OutputContentAddressed
                case "fixed":
                    outputMode = derivation.OutputFixed
                default:
                    return nil, fmt.Errorf("derivation: invalid outputMode string %s", outputModeS)
                }

                var fixedHashSha256 []byte
                if outputMode == derivation.OutputFixed {
                    // get fixedHash arg
                    fixedHashT := argAttrs["fixedHash"]
                    if fixedHashT == nil {
                        return nil, fmt.Errorf("derivation called with missing argument fixedHash")
                    }
                    fixedHashF, err := ip.Force(fixedHashT)
                    if err != nil {
                        return nil, err
                    }
                    fixedHash, isString := fixedHashF.(string)
                    if !isString {
                        return nil, fmt.Errorf("derivation called with invalid type for fixedHash arg %T: %v", fixedHashF, fixedHashF)
                    }
                    if !strings.HasPrefix(fixedHash, "sha256-") {
                        return nil, fmt.Errorf("derivation called with unsupported fixedHash format %s", fixedHash)
                    }
                    fixedHashSha256, err = base64.StdEncoding.DecodeString(strings.TrimPrefix(fixedHash, "sha256-"))
                    if err != nil {
                        return nil, err
                    }
                }

                drv, err := derivation.New(name, outputs, outputMode, fixedHashSha256, runtimeInputs)
                if err != nil {
                    return nil, err
                }
                err = drv.AddBuildInfo(buildPlatform, builder, args, buildInputs, env, buildFiles)
                if err != nil {
                    return nil, err
                }

                return drv, nil
            })),
            "outPathPlaceholder": interpreter.NewThunkEvaluated(interpreter.NewFnDefExternal(func(ip *interpreter.Interpreter, argT *interpreter.Thunk) (any, error) {
                argDerivationT := argT

                return interpreter.NewFnDefExternal(func(ip *interpreter.Interpreter, argT *interpreter.Thunk) (any, error) {
                    argOutputNameT := argT

                    argDerivationF, err := ip.Force(argDerivationT)
                    if err != nil {
                        return nil, err
                    }
                    derivation, isDerivation := argDerivationF.(*derivation.Derivation)
                    if !isDerivation {
                        return nil, fmt.Errorf("outPathPlaceholder: called! with unknown arg of type %T: %v", argDerivationF, argDerivationF)
                    }

                    argOutputNameF, err := ip.Force(argOutputNameT)
                    if err != nil {
                        return nil, err
                    }
                    outputName, isString := argOutputNameF.(string)
                    if !isString {
                        return nil, fmt.Errorf("outPathPlaceholder: called! with unknown arg of type %T: %v", argOutputNameF, argOutputNameF)
                    }

                    hash, err := derivation.OutputPlaceholderHash(outputName)
                    if err != nil {
                        return nil, err
                    }

                    return hash, nil
                }), nil
            })),
            "selfOutPathPlaceholder": interpreter.NewThunkEvaluated(interpreter.NewFnDefExternal(func(ip *interpreter.Interpreter, argT *interpreter.Thunk) (any, error) {
                argOutputNameF, err := ip.Force(argT)
                if err != nil {
                    return nil, err
                }
                outputName, isString := argOutputNameF.(string)
                if !isString {
                    return nil, fmt.Errorf("selfOutPathPlaceholder: called! with unknown arg of type %T: %v", argOutputNameF, argOutputNameF)
                }

                hash, err := derivation.SelfOutputPlaceholderHash(outputName)
                if err != nil {
                    return nil, err
                }

                return hash, nil
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
