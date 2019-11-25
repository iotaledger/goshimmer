package saltmanager

import (
	"github.com/iotaledger/goshimmer/plugins/autopeering/types/salt"
	"github.com/iotaledger/hive.go/events"
)

var Events = struct {
	UpdatePublicSalt  *events.Event
	UpdatePrivateSalt *events.Event
}{
	UpdatePublicSalt:  events.NewEvent(saltCaller),
	UpdatePrivateSalt: events.NewEvent(saltCaller),
}

func saltCaller(handler interface{}, params ...interface{}) {
	handler.(func(*salt.Salt))(params[0].(*salt.Salt))
}
