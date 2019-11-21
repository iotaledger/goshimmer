package autopeering

import (
	"encoding/base64"
	"strings"

	"github.com/iotaledger/autopeering-sim/peer"
	"github.com/iotaledger/goshimmer/packages/errors"
	"github.com/iotaledger/goshimmer/plugins/autopeering/parameters"
)

func parseEntryNodes() (result []*peer.Peer, err error) {
	for _, entryNodeDefinition := range strings.Fields(*parameters.ENTRY_NODES.Value) {
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

		result = append(result, peer.NewPeer(pubKey, parts[1]))
	}

	return result, nil
}
