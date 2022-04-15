package drng

import (
	"fmt"

	"github.com/cockroachdb/errors"
	"github.com/iotaledger/hive.go/serix"

	"github.com/iotaledger/goshimmer/packages/tangle/payload"
)

func init() {
	err := serix.DefaultAPI.RegisterTypeSettings(new(CollectiveBeaconPayload), serix.TypeSettings{}.WithObjectCode(new(CollectiveBeaconPayload).Type()))
	if err != nil {
		panic(fmt.Errorf("error registering Transaction type settings: %w", err))
	}
	err = serix.DefaultAPI.RegisterInterfaceObjects((*payload.Payload)(nil), new(CollectiveBeaconPayload))
	if err != nil {
		panic(fmt.Errorf("error registering Transaction as Payload interface: %w", err))
	}
}

const (
	// ObjectName defines the name of the dRNG object.
	ObjectName = "dRNG"
)

// region Payload implementation ///////////////////////////////////////////////////////////////////////////////////////

// PayloadType defines the type of the drng payload.
var PayloadType = payload.NewType(111, ObjectName, func(data []byte) (payload payload.Payload, err error) {
	var consumedBytes int
	payload, consumedBytes, err = CollectiveBeaconPayloadFromBytes(data)
	if err != nil {
		return nil, err
	}
	if consumedBytes != len(data) {
		return nil, errors.New("not all payload bytes were consumed")
	}

	return
})

// // endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
