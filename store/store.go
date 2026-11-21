package store

import "os"
import "path"
import "path/filepath"
import "errors"
import "encoding/base32"
import "io"
import "fmt"
import "crypto/sha256"
import "hash"
import "encoding/binary"

type Store struct {
	basePath string
}

// hash suitable for things that belong into the store (returns the hasher it created, either you call Sum(nil) on it right away, or you hash extra things!)
// similar general idea to Nix NAR
func Hash(path string) (hash.Hash, error) {
	hasher := sha256.New()
	if err := hashInt(path, "", hasher); err != nil {
		return nil, err
	}
	return hasher, nil
}

func hashInt(basePath string, archivePath string, hasher hash.Hash) error {
	fullPath := path.Join(basePath, archivePath)
	fullStat, err := os.Lstat(fullPath)
	if err != nil {
		return err
	}
	hashFile := func (basePath string, archivePath string, fullStat os.FileInfo, hasher hash.Hash) error {
		fullPath := path.Join(basePath, archivePath)
		if fullStat.Mode().IsRegular() {
			descStr := archivePath + "RegularFile"
			if isExecutable(fullStat.Mode()) {
				descStr += "Executable"
			}

			in, err := os.Open(fullPath)
			if err != nil {
				return err
			}
			defer in.Close()

			if err := binary.Write(hasher, binary.LittleEndian, uint64(len(descStr))); err != nil {
				return err
			}
			if _, err := io.WriteString(hasher, descStr); err != nil {
				return err
			}

			if err := binary.Write(hasher, binary.LittleEndian, uint64(fullStat.Size())); err != nil {
				return err
			}
			if _, err := io.Copy(hasher, in); err != nil {
				return err
			}

			return nil
		}
		switch fullStat.Mode() {
		// TODO: handle symlinks here
		default:
			return fmt.Errorf("hashFile unsupported mode: %v", fullStat.Mode())
		}
	}
	if fullStat.Mode().IsDir() {
		// Why does this work? ReadDir sorts entries by name!
		entries, err := os.ReadDir(fullPath)
		if err != nil {
			return err
		}
		descStr := archivePath + "Directory"
		if err := binary.Write(hasher, binary.LittleEndian, uint64(len(descStr))); err != nil {
			return err
		}
		if _, err := io.WriteString(hasher, descStr); err != nil {
			return err
		}
		if err := binary.Write(hasher, binary.LittleEndian, uint64(len(entries))); err != nil {
				return err
			}
		for _, entry := range entries {
			archivePathF := path.Join(archivePath, entry.Name())
			fullPathF := path.Join(basePath, archivePathF)

			fullFStat, err := os.Lstat(fullPathF)
			if err != nil {
				return err
			}

			if fullFStat.Mode().IsDir() {
				if err := hashInt(basePath, archivePathF, hasher); err != nil {
					return err
				}
			} else {
				if err := hashFile(basePath, archivePathF, fullFStat, hasher); err != nil {
					return err
				}
			}
		}
		return nil
	} else {
		return hashFile(basePath, archivePath, fullStat, hasher)
	}
	return nil
}

func New(basePath string) (*Store, error) {
	abspath, err := filepath.Abs(basePath)
	if err != nil {
		return nil, err
	}
	st := &Store{basePath: abspath}
	if st.check() != nil {
		if err := storeInit(st.basePath); err != nil {
			return nil, err
		}
		if err := st.check(); err != nil {
			return nil, err
		}
	}
	return st, nil
}

func isExecutable(mode os.FileMode) bool {
	return (mode.Perm() & ((1 << 0) | (1 << 3) | (1 << 6))) != 0
}

func copyToStore(src string, dst string) error {
	// with help from https://stackoverflow.com/a/56314145 - Posted by Oleg Neumyvakin, modified by community. See post 'Timeline' for change history - Retrieved 2026-11-20, License - CC BY-SA 4.0
	srcStat, err := os.Lstat(src)
	if err != nil {
		return err
	}
	copyFile := func (src string, dst string, srcStat os.FileInfo) error {
		if srcStat.Mode().IsRegular() {
			var dstPerm os.FileMode = 0444 // r--r--r--
			// executable?
			if isExecutable(srcStat.Mode()) {
				dstPerm = 0555 // r-xr-xr-x
			}

			out, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE, dstPerm)
			if err != nil {
				return err
			}
			defer out.Close()

			in, err := os.Open(src)
			if err != nil {
				return err
			}
			defer in.Close()

			_, err = io.Copy(out, in)
			if err != nil {
				return err
			}

			return nil
		}
		switch srcStat.Mode() {
		// TODO: handle symlinks here
		default:
			return fmt.Errorf("copyFile unsupported mode: %v", srcStat.Mode())
		}
	}
	if srcStat.Mode().IsDir() {
		entries, err := os.ReadDir(src)
		if err != nil {
			return err
		}
		if err := os.Mkdir(dst, 0755 /* rwxr-xr-x */); err != nil {
			return err
		}
		for _, entry := range entries {
			srcF := path.Join(src, entry.Name())
			dstF := path.Join(dst, entry.Name())

			srcFStat, err := os.Lstat(srcF)
			if err != nil {
				return err
			}

			if srcFStat.Mode().IsDir() {
				if err := copyToStore(srcF, dstF); err != nil {
					return err
				}
			} else {
				if err := copyFile(srcF, dstF, srcFStat); err != nil {
					return err
				}
			}
		}
		return nil
	} else {
		return copyFile(src, dst, srcStat)
	}
}

func (st *Store) CopyToStore(source string, name string, outputName string, sha256sum []byte) (string, error) {
	p, err := st.computePath(name, outputName, sha256sum)
	if err != nil {
		return p, err
	}

	_, err = os.Lstat(p)
	if err == nil {
		return p, fmt.Errorf("attempted to copy to existing store path from: %s to: %s", source, p)
	}

	err = copyToStore(source, p)
	if err != nil {
		return p, err
	}

	return p, nil
}

func (st *Store) GetPath(name string, outputName string, sha256sum []byte) (string, error) {
	p, err := st.computePath(name, outputName, sha256sum)
	if err != nil {
		return p, err
	}
	_, err = os.Stat(p)
	if err != nil {
		return p, err
	}
	return p, nil
}

func (st *Store) computePath(name string, outputName string, sha256sum []byte) (string, error) {
	name, err := st.computeName(name, outputName, sha256sum)
	if err != nil {
		return name, err
	}
	return path.Join(st.basePath, "store", name), nil
}

func (st *Store) computeName(name string, outputName string, sha256sum []byte) (string, error) {
	if len(sha256sum) != 32 {
		return "", errors.New("sha256sum is not 32 bytes")
	}
	suffix := ""
	if outputName != "default" {
		suffix = "-" + outputName
	}
	// same alphabet as nix https://nix.dev/manual/nix/2.35/protocols/nix32.html
	return base32.NewEncoding("0123456789abcdfghijklmnpqrsvwxyz").WithPadding(base32.NoPadding).EncodeToString(sha256sum) + "-" + name + suffix, nil
}

func (st *Store) check() error {
	_, err := os.Stat(st.basePath)
	if err != nil {
		return err
	}
	_, err = os.Stat(path.Join(st.basePath, "store"))
	if err != nil {
		return err
	}
	return nil
}

func storeInit(basePath string) error {
	// main dir
	if err := os.MkdirAll(basePath, 0755 /* rwxr-xr-x */); err != nil {
		return err
	}
	// /store
	if err := os.Mkdir(path.Join(basePath, "store"), 0755 /* rwxr-xr-x */); err != nil {
		return err
	}
	return nil
}
