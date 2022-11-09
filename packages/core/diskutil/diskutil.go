package diskutil

import (
	"crypto/sha256"
	"hash"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/iotaledger/hive.go/core/ioutils"
	"github.com/natefinch/atomic"
)

func ReplaceFile(src, dst string) (err error) {
	return atomic.ReplaceFile(src, dst)
}

type DiskUtil struct {
	basePath string
}

func New(basePath string, createIfAbsent ...bool) (newDiskUtil *DiskUtil) {
	if len(createIfAbsent) > 0 && createIfAbsent[0] {
		ioutils.CreateDirectory(basePath, 0o755)
	}
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

func (d *DiskUtil) Exists(path string) (exists bool) {
	if _, err := os.Stat(path); err != nil {
		if !os.IsNotExist(err) {
			panic(err)
		}

		return false
	}

	return true
}

func (d *DiskUtil) WriteFile(path string, data []byte) (err error) {
	return ioutil.WriteFile(path, data, 0o666)
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

func (d *DiskUtil) WithFile(filePath string, f func(file *os.File)) (err error) {
	file, err := os.Open(filePath)
	if err != nil {
		return
	}
	defer file.Close()

	f(file)

	return
}
