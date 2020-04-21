package drng

import (
	"encoding/hex"

	"github.com/iotaledger/goshimmer/packages/binary/drng"
	"github.com/iotaledger/goshimmer/packages/binary/drng/payload"
	"github.com/iotaledger/goshimmer/packages/binary/drng/payload/header"
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
	Instance *drng.DRNG
	log      *logger.Logger
)

func configure(*node.Plugin) {
	log = logger.NewLogger(name)

	// parse identities of the committee members
	committeeMembers, err := parseCommitteeMembers()
	if err != nil {
		log.Warnf("Invalid %s: %s", CfgDRNGCommitteeMembers, err)
	}

	// parse distributed public key of the committee
	var dpk []byte
	if str := config.Node.GetString(CfgDRNGDistributedPubKey); str != "" {
		bytes, err := hex.DecodeString(str)
		if err != nil {
			log.Warnf("Invalid %s: %s", CfgDRNGDistributedPubKey, err)
		}
		if l := len(bytes); l != cbPayload.PublicKeySize {
			log.Warnf("Invalid %s length: %d, need %d", CfgDRNGDistributedPubKey, l, cbPayload.PublicKeySize)
		}
		dpk = append(dpk, bytes...)
	}

	// configure committee
	committeeConf := &state.Committee{
		InstanceID:    config.Node.GetUint32(CfgDRNGInstanceID),
		Threshold:     uint8(config.Node.GetUint32(CfgDRNGThreshold)),
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
			if msg.Payload().Type() != payload.Type {
				return
			}
			if len(msg.Payload().Bytes()) < header.Length {
				return
			}
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
