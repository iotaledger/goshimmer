package utils

import (
	"os"
	"path/filepath"

	"github.com/iotaledger/hive.go/core/ioutils"
)

// Directory represents a directory on the disk.
type Directory struct {
	path string
}

// NewDirectory creates a new directory at the given path.
func NewDirectory(path string, createIfMissing ...bool) (newDirectory *Directory) {
	if len(createIfMissing) > 0 && createIfMissing[0] {
		if err := ioutils.CreateDirectory(path, defaultPermissions); err != nil {
			panic(err)
		}
	}

	return &Directory{
		path: path,
	}
}

// Path returns the absolute path that corresponds to the relative path.
func (d *Directory) Path(relativePathElements ...string) (path string) {
	return filepath.Join(append([]string{d.path}, relativePathElements...)...)
}

func (d *Directory) RemoveSubdir(name string) error {
	return os.RemoveAll(d.Path(name))
}

func (d *Directory) SubDirs() ([]string, error) {
	var dirs []string
	basePath := filepath.Base(d.path)

	err := filepath.WalkDir(d.path, func(path string, dirEntry os.DirEntry, err error) error {
		if err != nil {
			return err
		}

		switch filepath.Dir(path) {
		case ".":
			return nil
		case basePath:
			if dirEntry.IsDir() {
				dirs = append(dirs, dirEntry.Name())
			}
			return nil
		default:
			if dirEntry.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
	})

	return dirs, err
}

const defaultPermissions = 0o755
