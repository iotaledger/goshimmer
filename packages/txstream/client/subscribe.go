package client

import (
	"time"

	"github.com/iotaledger/goshimmer/packages/ledgerstate"
	"github.com/iotaledger/goshimmer/packages/txstream"
)

// Subscribe subscribes to real-time updates for the given address.
func (n *Client) Subscribe(addr ledgerstate.Address) {
	n.chSubscribe <- addr
}

// Unsubscribe unsubscribes the address.
func (n *Client) Unsubscribe(addr ledgerstate.Address) {
	n.chUnsubscribe <- addr
}

func (n *Client) subscriptionsLoop() {
	subscriptions := make(map[[ledgerstate.AddressLength]byte]ledgerstate.Address)

	ticker1m := time.NewTicker(time.Minute)
	defer ticker1m.Stop()

	for {
		select {
		case <-n.shutdown:
			return
		case addr := <-n.chSubscribe:
			addrBytes := addr.Array()
			if _, ok := subscriptions[addrBytes]; !ok {
				// new address is subscribed
				n.log.Infof("subscribed to address %s", addr.Base58())
				subscriptions[addrBytes] = addr
				n.sendSubscriptions(subscriptions)
			}
		case addr := <-n.chUnsubscribe:
			delete(subscriptions, addr.Array())
		case <-ticker1m.C:
			// send subscriptions once every minute
			n.sendSubscriptions(subscriptions)
		}
	}
}

func (n *Client) sendSubscriptions(subscriptions map[[ledgerstate.AddressLength]byte]ledgerstate.Address) {
	if len(subscriptions) == 0 {
		return
	}

	addrs := make([]ledgerstate.Address, len(subscriptions))
	{
		i := 0
		for _, addr := range subscriptions {
			addrs[i] = addr
			i++
		}
	}

	n.sendMessage(&txstream.MsgUpdateSubscriptions{Addresses: addrs})
}
