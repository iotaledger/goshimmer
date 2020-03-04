package coloredbalance

import (
	"strconv"

	"github.com/iotaledger/goshimmer/packages/binary/marshalutil"
	color2 "github.com/iotaledger/goshimmer/packages/binary/valuetransfers/payload/transfer/coloredbalance/color"
)

type ColoredBalance struct {
	color   color2.Color
	balance int64
}

func New(color color2.Color, balance int64) (result *ColoredBalance) {
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
		return color2.FromBytes(data)
	}); colorErr != nil {
		return nil, colorErr, marshalUtil.ReadOffset()
	} else {
		result.color = coinColor.(color2.Color)
	}

	result.balance, err = marshalUtil.ReadInt64()
	if err != nil {
		return
	}

	consumedBytes = marshalUtil.ReadOffset()

	return
}

func (balance *ColoredBalance) ToBytes() []byte {
	marshalUtil := marshalutil.New(color2.Length + marshalutil.UINT32_SIZE)

	marshalUtil.WriteBytes(balance.color.Bytes())
	marshalUtil.WriteInt64(balance.balance)

	return marshalUtil.Bytes()
}

func (balance *ColoredBalance) String() string {
	return strconv.FormatInt(balance.balance, 10) + " " + balance.color.String()
}
