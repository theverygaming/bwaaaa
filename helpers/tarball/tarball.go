package tarball

// Source - https://stackoverflow.com/a/57640231
// Posted by Victor Rusnac
// Retrieved 2026-11-20, License - CC BY-SA 4.0
// code cleaned up a bit and solved some possible security issues, but it's essentially the same :P
import (
    "archive/tar"
    "compress/gzip"
    "fmt"
    "io"
    "os"
	"path"
	"path/filepath"
	"strings"
)

func ExtractTarGz(destRoot string, gzipStream io.Reader) error {
	destRoot, err := filepath.Abs(destRoot)
	if err != nil {
		return fmt.Errorf("ExtractTarGz: filepath.Abs failed: %s", err.Error())
	}
    uncompressedStream, err := gzip.NewReader(gzipStream)
    if err != nil {
        return fmt.Errorf("ExtractTarGz: NewReader failed")
    }

    tarReader := tar.NewReader(uncompressedStream)

    for {
        header, err := tarReader.Next()

        if err == io.EOF {
            break
        }

        if err != nil {
            return fmt.Errorf("ExtractTarGz: Next() failed: %s", err.Error())
        }

		fdst, err := filepath.Abs(path.Join(destRoot, header.Name))
		if err != nil {
            return fmt.Errorf("ExtractTarGz: filepath.Abs failed: %s", err.Error())
        }
		if fdst != destRoot && !strings.HasPrefix(fdst, destRoot + string(os.PathSeparator)) {
			return fmt.Errorf("ExtractTarGz: encountered invalid name %s that would resolve to %s which is outside of %s", header.Name, fdst, destRoot)
		}

        switch header.Typeflag {
        case tar.TypeDir:
            if err := os.MkdirAll(fdst, 0755); err != nil {
                return fmt.Errorf("ExtractTarGz: Mkdir() failed: %s", err.Error())
            }
        case tar.TypeReg:
			if err := os.MkdirAll(filepath.Dir(fdst), 0755); err != nil {
                return fmt.Errorf("ExtractTarGz: Mkdir() failed: %s", err.Error())
            }
            outFile, err := os.OpenFile(fdst, os.O_WRONLY|os.O_CREATE|os.O_EXCL, header.FileInfo().Mode().Perm())
            if err != nil {
                return fmt.Errorf("ExtractTarGz: Create() failed: %s", err.Error())
            }
            if _, err := io.Copy(outFile, tarReader); err != nil {
                return fmt.Errorf("ExtractTarGz: Copy() failed: %s", err.Error())
            }
            outFile.Close()
        default:
            return fmt.Errorf(
                "ExtractTarGz: unknown type: %s in %s",
                header.Typeflag,
                header.Name)
        }
    }

	return nil
}
