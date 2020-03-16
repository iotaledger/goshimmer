package marshalutil

import (
	"fmt"
	"testing"
)

func Test(t *testing.T) {
	util := New(1)

	util.WriteBytes(make([]byte, UINT64_SIZE))
	util.WriteInt64(-12)
	util.WriteUint64(38)
	util.WriteUint64(38)

	fmt.Println(util.ReadBytes(UINT64_SIZE))
	fmt.Println(util.ReadInt64())
	fmt.Println(util.ReadUint64())
	fmt.Println(util.ReadUint64())

	fmt.Println(util)
}
