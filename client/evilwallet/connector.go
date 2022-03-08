package evilwallet

import (
	"sync"

	"github.com/iotaledger/goshimmer/client"
	"github.com/iotaledger/goshimmer/client/wallet"
	"github.com/iotaledger/goshimmer/packages/consensus/gof"
	"github.com/iotaledger/goshimmer/packages/jsonmodels"
	"github.com/iotaledger/goshimmer/packages/ledgerstate"
	"github.com/iotaledger/hive.go/identity"
)

type ServersStatus []*wallet.ServerStatus

type Clients interface {
	ServersStatuses() ServersStatus
	ServerStatus(cltIdx int) (status *wallet.ServerStatus, err error)
	Clients(...bool) []*client.GoShimmerAPI
	GetClients(numOfClt int) []*client.GoShimmerAPI
	GetClient() *client.GoShimmerAPI
	AddClient(url string, setters ...client.Option)
	RemoveClient(index int)
	PledgeID() *identity.ID

	// all API calls
	PostTransaction(tx *ledgerstate.Transaction, clt *client.GoShimmerAPI) (ledgerstate.TransactionID, error)
	GetUnspentOutputForAddress(addr ledgerstate.Address) *jsonmodels.WalletOutput
	GetTransactionGoF(txID string) gof.GradeOfFinality
	GetOutputGoF(outputID ledgerstate.OutputID) gof.GradeOfFinality
	SendFaucetRequest(address string) error
	GetTransactionOutputs(txID string) (outputs ledgerstate.Outputs, err error)
	GetTransaction(txID string) (resp *jsonmodels.Transaction, err error)
}

// Connector is responsible for handling connections with clients.
type Connector struct {
	clients []*client.GoShimmerAPI
	urls    []string

	// can be used in case we want all mana to be pledge to a specific node
	pledgeID *identity.ID
	// helper variable indicating which clt was recently used, useful for double, triple,... spends
	lastUsed int

	mu sync.Mutex
}

// NewConnector creates Connector from provided GoShimmerAPI urls.
func NewConnector(urls []string, setters ...client.Option) *Connector {
	clients := make([]*client.GoShimmerAPI, len(urls))
	for i, url := range urls {
		clients[i] = client.NewGoShimmerAPI(url, setters...)
	}

	return &Connector{
		clients:  clients,
		urls:     urls,
		lastUsed: -1,
	}
}

// ServersStatuses retrieves the connected server status for each client.
func (c *Connector) ServersStatuses() ServersStatus {
	status := make(ServersStatus, len(c.clients))

	for i := range c.clients {
		status[i], _ = c.ServerStatus(i)
	}
	return status
}

// ServerStatus retrieves the connected server status.
func (c *Connector) ServerStatus(cltIdx int) (status *wallet.ServerStatus, err error) {
	response, err := c.clients[cltIdx].Info()
	if err != nil {
		return nil, err
	}

	status.ID = response.IdentityID
	status.Synced = response.TangleTime.Synced
	status.Version = response.Version
	status.ManaDecay = response.ManaDecay
	status.DelegationAddress = response.ManaDelegationAddress
	return status, nil
}

// Clients returns list of all clients.
func (c *Connector) Clients(...bool) []*client.GoShimmerAPI {
	return c.clients
}

// GetClients returns the numOfClt client instances that were used the longest time ago.
func (c *Connector) GetClients(numOfClt int) []*client.GoShimmerAPI {
	c.mu.Lock()
	defer c.mu.Unlock()

	clts := make([]*client.GoShimmerAPI, numOfClt)

	for i := range clts {
		clts[i] = c.getClient()
	}
	return clts
}

// getClient returns the client instance that was used the longest time ago, not protected by mutex.
func (c *Connector) getClient() *client.GoShimmerAPI {
	if c.lastUsed == len(c.clients)-1 {
		c.lastUsed = 0
	} else {
		c.lastUsed++
	}
	return c.clients[c.lastUsed]
}

// GetClient returns the client instance that was used the longest time ago.
func (c *Connector) GetClient() *client.GoShimmerAPI {
	c.mu.Lock()
	defer c.mu.Unlock()

	return c.getClient()
}

// AddClient adds client to Connector based on provided GoShimmerAPI url.
func (c *Connector) AddClient(url string, setters ...client.Option) {
	c.mu.Lock()
	defer c.mu.Unlock()

	clt := client.NewGoShimmerAPI(url, setters...)
	c.clients = append(c.clients, clt)
}

// RemoveClient removes client with the provided index from the Connector.
func (c *Connector) RemoveClient(index int) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.clients = append(c.clients[:index], c.clients[index+1:]...)
}

// PledgeID returns the node ID that the mana will be pledging to.
func (c *Connector) PledgeID() *identity.ID {
	return c.pledgeID
}

// SetPledgeID sets the node ID that the mana will be pledging to.
func (c *Connector) SetPledgeID(id *identity.ID) {
	c.pledgeID = id
}

// SendFaucetRequest requests funds from the faucet and returns the faucet request message ID.
func (c *Connector) SendFaucetRequest(address string) (err error) {
	clt := c.GetClient()
	_, err = clt.SendFaucetRequest(address, -1)
	return
}

// PostTransaction sends a transaction to the Tangle via a given client.
func (c *Connector) PostTransaction(tx *ledgerstate.Transaction, clt *client.GoShimmerAPI) (txID ledgerstate.TransactionID, err error) {
	resp, err := clt.PostTransaction(tx.Bytes())
	if err != nil {
		return
	}
	txID, err = ledgerstate.TransactionIDFromBase58(resp.TransactionID)
	if err != nil {
		return
	}
	return
}

// GetUnspentOutputForAddress gets the first unspent outputs of a given address.
func (c *Connector) GetUnspentOutputForAddress(addr ledgerstate.Address) *jsonmodels.WalletOutput {
	clt := c.GetClient()
	resp, err := clt.PostAddressUnspentOutputs([]string{addr.Base58()})
	if err != nil {
		return nil
	}
	outputs := resp.UnspentOutputs[0].Outputs
	if len(outputs) > 0 {
		return &outputs[0]
	}
	return nil
}

// GetOutputGoF gets the first unspent outputs of a given address.
func (c *Connector) GetOutputGoF(outputID ledgerstate.OutputID) gof.GradeOfFinality {
	clt := c.GetClient()
	res, err := clt.GetOutputMetadata(outputID.Base58())
	if err != nil {
		return gof.None
	}

	return res.GradeOfFinality
}

// GetTransactionGoF returns the GoF of a given transaction ID.
func (c *Connector) GetTransactionGoF(txID string) gof.GradeOfFinality {
	clt := c.GetClient()
	resp, err := clt.GetTransactionMetadata(txID)
	if err != nil {
		return gof.None
	}
	return resp.GradeOfFinality
}

func (c *Connector) GetTransactionOutputs(txID string) (outputs ledgerstate.Outputs, err error) {
	clt := c.GetClient()
	resp, err := clt.GetTransaction(txID)
	if err != nil {
		return
	}
	for _, output := range resp.Outputs {
		out, err2 := output.ToLedgerstateOutput()
		if err2 != nil {
			return
		}
		outputs = append(outputs, out)
	}
	return
}

func (c *Connector) GetTransaction(txID string) (resp *jsonmodels.Transaction, err error) {
	clt := c.GetClient()
	resp, err = clt.GetTransaction(txID)
	if err != nil {
		return
	}
	return
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
