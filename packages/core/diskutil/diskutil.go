package diskutil

import (
	"crypto/sha256"
	"hash"
	"io"
	"io/ioutil"
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

func (d *DiskUtil) CopyFile(src, dst string) (err error) {
	srcFile, err := os.Open(src)
	if err != nil {
		return
	}
	defer srcFile.Close()

	dstFile, err := os.Create(dst)
	if err != nil {
		return
	}
	defer dstFile.Close()

	if _, err = io.Copy(dstFile, srcFile); err != nil {
		return
	}

	return dstFile.Sync()
}

func (d *DiskUtil) CreateDir(directoryPath string, perm ...os.FileMode) (err error) {
	if len(perm) > 0 {
		return os.MkdirAll(directoryPath, perm[0])
	}

	return os.MkdirAll(directoryPath, 0755)
}

func (d *DiskUtil) WriteFile(path string, data []byte) (err error) {
	return ioutil.WriteFile(path, data, 0666)
}

func (d *DiskUtil) Path(relativePathElements ...string) (path string) {
	return filepath.Join(append([]string{d.basePath}, relativePathElements...)...)
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
