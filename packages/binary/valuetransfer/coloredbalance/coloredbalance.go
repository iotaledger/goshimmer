package coloredbalance

import (
	"strconv"

	"github.com/iotaledger/goshimmer/packages/binary/marshalutil"
	"github.com/iotaledger/goshimmer/packages/binary/valuetransfer/coloredbalance/color"
)

type ColoredBalance struct {
	color   color.Color
	balance int64
}

func New(color color.Color, balance int64) (result *ColoredBalance) {
	result = &ColoredBalance{
		color:   color,
		balance: balance,
	}

	return
}

func FromBytes(bytes []byte) (result *ColoredBalance, err error, consumedBytes int) {
	result = &ColoredBalance{}

	marshalUtil := marshalutil.New(bytes)

	if coinColor, colorErr := marshalUtil.Parse(func(data []byte) (interface{}, error, int) {
		return color.FromBytes(data)
	}); colorErr != nil {
		return nil, colorErr, marshalUtil.ReadOffset()
	} else {
		result.color = coinColor.(color.Color)
	}

	result.balance, err = marshalUtil.ReadInt64()
	if err != nil {
		return
	}

	consumedBytes = marshalUtil.ReadOffset()

	return
}

// Parse is a wrapper for simplified unmarshaling in a byte stream using the marshalUtil package.
func Parse(marshalUtil *marshalutil.MarshalUtil) (*ColoredBalance, error) {
	if address, err := marshalUtil.Parse(func(data []byte) (interface{}, error, int) { return FromBytes(data) }); err != nil {
		return nil, err
	} else {
		return address.(*ColoredBalance), nil
	}
}

func (balance *ColoredBalance) Bytes() []byte {
	marshalUtil := marshalutil.New(color.Length + marshalutil.UINT32_SIZE)

	marshalUtil.WriteBytes(balance.color.Bytes())
	marshalUtil.WriteInt64(balance.balance)

	return marshalUtil.Bytes()
}

func (balance *ColoredBalance) String() string {
	return strconv.FormatInt(balance.balance, 10) + " " + balance.color.String()
}

// Length encodes the length of a marshaled ColoredBalance (the length of the color + 8 bytes for the balance).
const Length = color.Length + 8
