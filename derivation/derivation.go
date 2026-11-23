package derivation;

import "io"
import "crypto/sha256"
import "encoding/binary"
import "encoding/hex"
import "sort"
import "bytes"
import "errors"

type DerivationOutputMode int

const (
    OutputInputAddressed DerivationOutputMode = iota
    OutputContentAddressed
    OutputFixed // NOTE: fixed output derivations can only have a single output named "default"
)

type Derivation struct {
    name string
    outputs []string
    outputMode DerivationOutputMode
    fixedHashSha256 []byte // for outputMode=OutputFixed specifically
	runtimeInputs []*Derivation // build-time and runtime inputs
    buildInfo *DerivationBuildInfo // optional. Sometimes derivations may have no build information attached to them.
    hashCachedSha256 []byte // cached hash
}

func New(
	name string,
	outputs []string,
	outputMode DerivationOutputMode,
	fixedHashSha256 []byte, // nil for non-fixed derivation
	runtimeInputs []*Derivation, // build-time and runtime inputs
) (*Derivation, error) {
	if outputMode != OutputFixed && fixedHashSha256 != nil {
		return nil, errors.New("a non-fixed output derivation may not have a fixed hash")
	} else if outputMode == OutputFixed && len(fixedHashSha256) != 32 {
		return nil, errors.New("fixed output derivation: missing or invalid hash")
	}
	// FIXME: deduplicate runtimeInputs (by hash)
	drv := &Derivation{
		name: name,
		outputs: outputs,
		outputMode: outputMode,
		fixedHashSha256: fixedHashSha256,
		runtimeInputs: runtimeInputs,
		buildInfo: nil,
		hashCachedSha256: nil,
	}
	return drv, nil
}

type DerivationBuildInfo struct {
    buildPlatform string // platform the build is executed on
    builder string // builder executable path
    args []string // builder args
    buildInputs []*Derivation // build-time only inputs
    env map[string]string
    buildFiles map[string]string // content will be copied into build env as files and pointed to by env vars F_<caps_name>
}

func (drv *Derivation) AddBuildInfo(
	platform string, // platform the build is executed on
	builder string, // builder executable path
	args []string, // builder args
	buildInputs []*Derivation, // build-time only inputs
	env map[string]string,
	buildFiles map[string]string, // content will be copied into build env as files and pointed to by env vars F_<caps_name>
) error {
	if drv.hashCachedSha256 != nil {
		return errors.New("AddBuildInfo called but derivation already has a cached hash")
	}
	if drv.buildInfo != nil {
		return errors.New("AddBuildInfo called but derivation already has buildInfo")
	}
	// FIXME: deduplicate buildInputs (by hash)
	// FIXME: ensure that fixed output derivations can only have a single output named "default"
	drv.buildInfo = &DerivationBuildInfo{
		buildPlatform: platform,
		builder: builder,
		args: args,
		buildInputs: buildInputs,
		env: env,
		buildFiles: buildFiles,
	}
	return nil
}

func (drv *Derivation) Hash() ([]byte, error) {
    if len(drv.hashCachedSha256) == 32 {
        return drv.hashCachedSha256, nil
    }
    hasher := sha256.New()
    if _, err := io.WriteString(hasher, "Derivation:"); err != nil {
        return nil, err
    }
    if _, err := io.WriteString(hasher, drv.name); err != nil {
        return nil, err
    }
    if err := binary.Write(hasher, binary.LittleEndian, uint64(len(drv.outputs))); err != nil {
        return nil, err
    }
    for _, i := range drv.outputs {
        if _, err := io.WriteString(hasher, i); err != nil {
            return nil, err
        }
    }
    if err := binary.Write(hasher, binary.LittleEndian, uint64(drv.outputMode)); err != nil {
        return nil, err
    }
    if drv.outputMode == OutputFixed {
        if _, err := hasher.Write(drv.fixedHashSha256); err != nil {
            return nil, err
        }
    }
	if err := binary.Write(hasher, binary.LittleEndian, uint64(len(drv.runtimeInputs))); err != nil {
		return nil, err
	}
	runtimeInputsHashes := [][]byte{}
	for _, i := range drv.runtimeInputs {
		hash, err := i.Hash()
		if err != nil {
			return nil, err
		}
		runtimeInputsHashes = append(runtimeInputsHashes, hash)
	}
	sort.Slice(runtimeInputsHashes, func(i int, j int) bool {
		return bytes.Compare(runtimeInputsHashes[i], runtimeInputsHashes[j]) < 0
	})
	for _, i := range runtimeInputsHashes {
		if _, err := hasher.Write(i); err != nil {
			return nil, err
		}
	}
    if drv.buildInfo != nil {
        if _, err := io.WriteString(hasher, drv.buildInfo.buildPlatform); err != nil {
            return nil, err
        }
        if _, err := io.WriteString(hasher, drv.buildInfo.builder); err != nil {
            return nil, err
        }
        if err := binary.Write(hasher, binary.LittleEndian, uint64(len(drv.buildInfo.args))); err != nil {
            return nil, err
        }
        for _, i := range drv.buildInfo.args {
            if _, err := io.WriteString(hasher, i); err != nil {
                return nil, err
            }
        }
        if err := binary.Write(hasher, binary.LittleEndian, uint64(len(drv.buildInfo.buildInputs))); err != nil {
            return nil, err
        }
        buildInputsHashes := [][]byte{}
        for _, i := range drv.buildInfo.buildInputs {
            hash, err := i.Hash()
            if err != nil {
                return nil, err
            }
            buildInputsHashes = append(buildInputsHashes, hash)
        }
        sort.Slice(buildInputsHashes, func(i int, j int) bool {
            return bytes.Compare(buildInputsHashes[i], buildInputsHashes[j]) < 0
        })
        for _, i := range buildInputsHashes {
            if _, err := hasher.Write(i); err != nil {
                return nil, err
            }
        }
        if err := binary.Write(hasher, binary.LittleEndian, uint64(len(drv.buildInfo.env))); err != nil {
            return nil, err
        }
        for key, value := range drv.buildInfo.env {
            if _, err := io.WriteString(hasher, key); err != nil {
                return nil, err
            }
            if _, err := io.WriteString(hasher, value); err != nil {
                return nil, err
            }
        }
        if err := binary.Write(hasher, binary.LittleEndian, uint64(len(drv.buildInfo.buildFiles))); err != nil {
            return nil, err
        }
        for key, value := range drv.buildInfo.buildFiles {
            if _, err := io.WriteString(hasher, key); err != nil {
                return nil, err
            }
            if _, err := io.WriteString(hasher, value); err != nil {
                return nil, err
            }
        }
    }

    hash := hasher.Sum(nil)
    drv.hashCachedSha256 = hash
    return hash, nil
}

func (drv *Derivation) OutputPlaceholderHash(outputName string) (string, error) {
    var output string
    hash, err := drv.Hash()
    if err != nil {
        return output, err
    }
    hasher := sha256.New()
    if _, err := io.WriteString(hasher, "DerivationOutput:"); err != nil {
        return output, err
    }
    if _, err := io.WriteString(hasher, outputName); err != nil {
        return output, err
    }
    if _, err := hasher.Write(hash); err != nil {
        return output, err
    }
    outputHash := hasher.Sum(nil)
    output = hex.EncodeToString(outputHash)
    return output, nil
}

func SelfOutputPlaceholderHash(outputName string) (string, error) {
    hasher := sha256.New()
    var output string
    if _, err := io.WriteString(hasher, "SelfDerivationOutput:"); err != nil {
        return output, err
    }
    if _, err := io.WriteString(hasher, outputName); err != nil {
        return output, err
    }
    selfOutputHash := hasher.Sum(nil)
    output = hex.EncodeToString(selfOutputHash)
    return output, nil
}
