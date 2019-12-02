package autopeering

import (
	"encoding/base64"
	"strings"

	"github.com/iotaledger/autopeering-sim/peer"
	"github.com/iotaledger/autopeering-sim/peer/service"
	"github.com/iotaledger/goshimmer/packages/errors"
	"github.com/iotaledger/hive.go/parameter"
)

func parseEntryNodes() (result []*peer.Peer, err error) {
	for _, entryNodeDefinition := range strings.Fields(parameter.NodeConfig.GetString(CFG_ENTRY_NODES)) {
		if entryNodeDefinition == "" {
			continue
		}

		parts := strings.Split(entryNodeDefinition, "@")
		if len(parts) != 2 {
			return nil, errors.New("parseMaster")
		}
		pubKey, err := base64.StdEncoding.DecodeString(parts[0])
		if err != nil {
			return nil, errors.Wrap(err, "parseMaster")
		}

		services := service.New()
		services.Update(service.PeeringKey, "udp", parts[1])

		result = append(result, peer.NewPeer(pubKey, services))
	}

	return result, nil
}
