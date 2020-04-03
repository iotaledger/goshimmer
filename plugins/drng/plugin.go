package drng

import (
	"encoding/hex"

	"github.com/iotaledger/goshimmer/packages/binary/drng"
	"github.com/iotaledger/goshimmer/packages/binary/drng/payload"
	"github.com/iotaledger/goshimmer/packages/binary/drng/state"
	cbPayload "github.com/iotaledger/goshimmer/packages/binary/drng/subtypes/collectiveBeacon/payload"
	"github.com/iotaledger/goshimmer/packages/binary/messagelayer/message"
	"github.com/iotaledger/goshimmer/packages/binary/messagelayer/tangle"
	"github.com/iotaledger/goshimmer/plugins/config"
	"github.com/iotaledger/goshimmer/plugins/messagelayer"
	"github.com/iotaledger/hive.go/events"
	"github.com/iotaledger/hive.go/logger"
	"github.com/iotaledger/hive.go/marshalutil"
	"github.com/iotaledger/hive.go/node"
)

const name = "DRNG" // name of the plugin

var PLUGIN = node.NewPlugin(name, node.Enabled, configure, run)

var (
	Instance *drng.Instance
	log      *logger.Logger
)

func configure(*node.Plugin) {
	log = logger.NewLogger(name)

	// parse identities of the committee members
	committeeMembers, err := parseCommitteeMembers()
	if err != nil {
		log.Fatalf("Invalid %s: %s", CFG_COMMITTEE_MEMBERS, err)
	}

	// parse distributed public key of the committee
	var dpk []byte
	if str := config.Node.GetString(CFG_DISTRIBUTED_PUB_KEY); str != "" {
		bytes, err := hex.DecodeString(str)
		if err != nil {
			log.Fatalf("Invalid %s: %s", CFG_DISTRIBUTED_PUB_KEY, err)
		}
		if l := len(bytes); l != cbPayload.PublicKeySize {
			log.Fatalf("Invalid %s length: %d, need %d", CFG_DISTRIBUTED_PUB_KEY, l, cbPayload.PublicKeySize)
		}
		dpk = append(dpk, bytes...)
	}

	// configure committee
	committeeConf := &state.Committee{
		InstanceID:    config.Node.GetUint32(CFG_INSTANCE_ID),
		Threshold:     uint8(config.Node.GetUint32(CFG_THRESHOLD)),
		DistributedPK: dpk,
		Identities:    committeeMembers,
	}

	Instance = drng.New(state.SetCommittee(committeeConf))

	configureEvents()
}

func run(*node.Plugin) {}

func configureEvents() {
	messagelayer.Tangle.Events.TransactionSolid.Attach(events.NewClosure(func(cachedMessage *message.CachedMessage, cachedMessageMetadata *tangle.CachedMessageMetadata) {
		cachedMessageMetadata.Release()

		cachedMessage.Consume(func(msg *message.Message) {
			marshalUtil := marshalutil.New(msg.Payload().Bytes())
			parsedPayload, err := payload.Parse(marshalUtil)
			if err != nil {
				//TODO: handle error
				log.Info(err)
				return
			}
			if err := Instance.Dispatch(msg.IssuerPublicKey(), msg.IssuingTime(), parsedPayload); err != nil {
				//TODO: handle error
				log.Info(err)
				return
			}
			log.Info(Instance.State.Randomness())
		})
	}))

}
