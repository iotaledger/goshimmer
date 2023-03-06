package storable_test

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/iotaledger/goshimmer/packages/core/storable"
	"github.com/iotaledger/hive.go/serializer/v2/serix"
)

func TestSlice(t *testing.T) {
	filePath := filepath.Join(t.TempDir(), "node.settings")

	entryLen, err := determineElementLength()
	require.NoError(t, err)

	slice, err := storable.NewSlice[Element](filePath, entryLen)
	require.NoError(t, err)

	var (
		element1        = NewElement(1)
		element2        = NewElement(2)
		element3        = NewElement(3)
		element4        = NewElement(4)
		rewriteElement1 = NewElement(5)
	)

	err = slice.Set(1, element1)
	require.NoError(t, err)
	err = slice.Set(2, element2)
	require.NoError(t, err)
	err = slice.Set(3, element3)
	require.NoError(t, err)

	getElement1, err := slice.Get(1)
	require.NoError(t, err)
	require.EqualValues(t, element1, getElement1)
	getElement3, err := slice.Get(3)
	require.NoError(t, err)
	require.EqualValues(t, element3, getElement3)

	//rewrite element
	err = slice.Set(1, rewriteElement1)
	require.NoError(t, err)

	getElement1, err = slice.Get(1)
	require.NoError(t, err)
	require.EqualValues(t, rewriteElement1, getElement1)

	err = slice.Close()
	require.NoError(t, err)

	// reopen file
	slice, err = storable.NewSlice[Element](filePath, entryLen)
	require.NoError(t, err)

	getElement1, err = slice.Get(1)
	require.NoError(t, err)
	require.EqualValues(t, rewriteElement1, getElement1)

	err = slice.Set(4, element4)
	require.NoError(t, err)

	getElement4, err := slice.Get(4)
	require.NoError(t, err)
	require.EqualValues(t, element4, getElement4)

	err = slice.Close()
	require.NoError(t, err)
}

type Element struct {
	Number int64 `serix:"1"`
}

func NewElement(num int64) (e *Element) {
	return &Element{
		Number: num,
	}
}

func (e *Element) FromBytes(bytes []byte) (int, error) {
	return serix.DefaultAPI.Decode(context.Background(), bytes, e)
}

func (e *Element) Bytes() ([]byte, error) {
	return serix.DefaultAPI.Encode(context.Background(), *e)
}

func determineElementLength() (length int, err error) {
	b, err := NewElement(0).Bytes()
	if err != nil {
		return 0, err
	}

	return len(b), nil
}
