package client

import (
	"github.com/iotaledger/goshimmer/packages/ledgerstate"
	"github.com/iotaledger/goshimmer/packages/txstream"
)

// RequestBacklog requests the backlog for a given address.
func (n *Client) RequestBacklog(addr ledgerstate.Address) {
	n.sendMessage(&txstream.MsgGetBacklog{Address: addr})
}

// RequestConfirmedTransaction requests a specific confirmed transaction.
func (n *Client) RequestConfirmedTransaction(addr ledgerstate.Address, txid ledgerstate.TransactionID) {
	n.sendMessage(&txstream.MsgGetConfirmedTransaction{
		Address: addr,
		TxID:    txid,
	})
}

// RequestTxInclusionState requests the inclusion state of a transaction.
func (n *Client) RequestTxInclusionState(addr ledgerstate.Address, txid ledgerstate.TransactionID) {
	n.sendMessage(&txstream.MsgGetTxInclusionState{
		Address: addr,
		TxID:    txid,
	})
}

// RequestConfirmedOutput requests a specific confirmed output.
func (n *Client) RequestConfirmedOutput(addr ledgerstate.Address, outputID ledgerstate.OutputID) {
	n.sendMessage(&txstream.MsgGetConfirmedOutput{
		Address:  addr,
		OutputID: outputID,
	})
}

// RequestUnspentAliasOutput requests the unique unspent alias output for the given AliasAddress.
func (n *Client) RequestUnspentAliasOutput(addr *ledgerstate.AliasAddress) {
	n.sendMessage(&txstream.MsgGetUnspentAliasOutput{
		AliasAddress: addr,
	})
}

// PostTransaction posts a transaction to the ledger.
func (n *Client) PostTransaction(tx *ledgerstate.Transaction) {
	n.sendMessage(&txstream.MsgPostTransaction{Tx: tx})
}
