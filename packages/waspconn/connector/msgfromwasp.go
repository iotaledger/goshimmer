package connector

// Copyright 2020 IOTA Stiftung
// SPDX-License-Identifier: Apache-2.0

import (
	"github.com/iotaledger/goshimmer/packages/tangle"
	"github.com/iotaledger/goshimmer/packages/waspconn"
)

// process messages received from the Wasp
func (wconn *WaspConnector) processMsgDataFromWasp(data []byte) {
	var msg interface{}
	var err error
	if msg, err = waspconn.DecodeMsg(data, false); err != nil {
		wconn.log().Errorf("DecodeMsg: %v", err)
		return
	}
	switch msg := msg.(type) {
	case *waspconn.WaspMsgChunk:
		finalMsg, err := wconn.messageChopper.IncomingChunk(msg.Data, tangle.MaxMessageSize, waspconn.ChunkMessageHeaderSize)
		if err != nil {
			wconn.log().Errorf("DecodeMsg: %v", err)
			return
		}
		if finalMsg != nil {
			wconn.processMsgDataFromWasp(finalMsg)
		}

	case *waspconn.WaspToNodePostTransactionMsg:
		wconn.postTransaction(msg.Tx)

	case *waspconn.WaspToNodeUpdateSubscriptionsMsg:
		newAddrs := wconn.setSubscriptions(msg.ChainAddresses)
		// send backlogs of newly subscribed addresses
		for _, addr := range newAddrs {
			wconn.getBacklog(addr)
		}

	case *waspconn.WaspToNodeGetConfirmedTransactionMsg:
		wconn.pushTransaction(msg.TxID, msg.ChainAddress)

	case *waspconn.WaspToNodeGetTxInclusionStateMsg:
		wconn.getTxInclusionState(msg.TxID, msg.ChainAddress)

	case *waspconn.WaspToNodeGetBacklogMsg:
		wconn.getBacklog(msg.ChainAddress)

	case *waspconn.WaspToNodeSetIdMsg:
		wconn.SetId(msg.WaspID)

	default:
		panic("wrong msg type")
	}
}
