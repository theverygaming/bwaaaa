package store

import "os"
import "path"

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
