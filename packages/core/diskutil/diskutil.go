package diskutil

import (
	"crypto/sha256"
	"hash"
	"io"
	"os"
	"path/filepath"
)

type DiskUtil struct {
	basePath string
}

func New(basePath string) (newDiskUtil *DiskUtil) {
	return &DiskUtil{
		basePath: basePath,
	}
}

func (d *DiskUtil) RelativePath(pathElements ...string) (path string) {
	return filepath.Join(append([]string{d.basePath}, pathElements...)...)
}

func (d *DiskUtil) FileChecksum(filePath string, hash ...hash.Hash) (checksum [32]byte, err error) {
	file, err := os.Open(filePath)
	if err != nil {
		return
	}
	defer file.Close()

	if len(hash) == 0 {
		hash = append(hash, sha256.New())
	}

	if _, err = io.Copy(hash[0], file); err != nil {
		return
	}

	copy(checksum[:], hash[0].Sum(nil))

	return
}

func (d *DiskUtil) WithFile(filePath string, f func(file *os.File) error) (err error) {
	file, err := os.Open(filePath)
	if err != nil {
		return
	}
	defer file.Close()

	return f(file)
}
