package storable

import (
	"bytes"
	"os"

	"github.com/natefinch/atomic"
	"github.com/pkg/errors"
)

// Struct contains logic that can be embedded in other structs to make them persist-able to disk.
type Struct[A any, B StructConstraint[A, B]] struct {
	object   B
	filePath string
}

// InitStruct makes the passed in object persist-able in the given file.
func InitStruct[A any, B StructConstraint[A, B]](object B, filePath string) B {
	return object.InitStruct(object, filePath)
}

// InitStruct initializes the Struct with the necessary information and reads the file. Read errors (i.e. when creating
// the file the first time) will be ignored and the object will have its default values.
func (s *Struct[A, B]) InitStruct(object B, filePath string) B {
	s.object = object
	s.filePath = filePath

	_ = s.FromFile(filePath)

	return object
}

// FromFile fills the instance with the serialized information from the given file.
func (s *Struct[A, B]) FromFile(fileName ...string) (err error) {
	filePath := s.filePath
	if len(fileName) > 0 {
		filePath = fileName[0]
	}

	readBytes, err := os.ReadFile(filePath)
	if err != nil {
		return errors.Errorf("failed to read file %s: %s", filePath, err)
	}

	if _, err = s.object.FromBytes(readBytes); err != nil {
		return errors.Errorf("failed to deserialize object stored in file %s: %s", filePath, err)
	}

	return nil
}

// ToFile serializes the content of the instance to the named file.
func (s *Struct[A, B]) ToFile(fileName ...string) (err error) {
	bytesToWrite, err := s.object.Bytes()
	if err != nil {
		return errors.Wrap(err, "failed to serialize object")
	}

	if len(fileName) != 0 {
		return atomic.WriteFile(fileName[0], bytes.NewReader(bytesToWrite))
	}

	return atomic.WriteFile(s.filePath, bytes.NewReader(bytesToWrite))
}

// FilePath returns the path that this Struct is associated to.
func (s *Struct[A, B]) FilePath() (filePath string) {
	return s.filePath
}
