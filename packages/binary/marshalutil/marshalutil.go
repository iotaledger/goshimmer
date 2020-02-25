package marshalutil

import (
	"fmt"
)

type MarshalUtil struct {
	bytes       []byte
	readOffset  int
	writeOffset int
	size        int
}

func New(args ...interface{}) *MarshalUtil {
	switch argsCount := len(args); argsCount {
	case 0:
		return &MarshalUtil{
			bytes: make([]byte, 1024),
			size:  0,
		}
	case 1:
		switch param := args[0].(type) {
		case int:
			return &MarshalUtil{
				bytes: make([]byte, param),
				size:  param,
			}
		case []byte:
			return &MarshalUtil{
				bytes: param,
				size:  len(param),
			}
		default:
			panic(fmt.Errorf("illegal argument type %T in marshalutil.New(...)", param))
		}
	default:
		panic(fmt.Errorf("illegal argument count %d in marshalutil.New(...)", argsCount))
	}
}

func (util *MarshalUtil) WriteSeek(offset int) {
	util.writeOffset = offset
}

func (util *MarshalUtil) ReadSeek(offset int) {
	util.readOffset = offset
}

func (util *MarshalUtil) Bytes() []byte {
	return util.bytes[:util.size]
}

func (util *MarshalUtil) checkReadCapacity(length int) (readEndOffset int, err error) {
	readEndOffset = util.readOffset + length

	if readEndOffset > util.size {
		err = fmt.Errorf("tried to read %d bytes from %d bytes input", readEndOffset, util.size)
	}

	return
}

func (util *MarshalUtil) expandWriteCapacity(length int) (writeEndOffset int) {
	writeEndOffset = util.writeOffset + length

	if writeEndOffset > util.size {
		extendedBytes := make([]byte, writeEndOffset-util.size)
		util.bytes = append(util.bytes, extendedBytes...)
		util.size = writeEndOffset
	}

	return
}
