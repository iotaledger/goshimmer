package utils

import (
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

const defaultPermissions = 0o755
