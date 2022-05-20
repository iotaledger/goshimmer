package evilwallet

import (
	"sync"

	"github.com/iotaledger/hive.go/identity"

	"github.com/iotaledger/goshimmer/client"
	"github.com/iotaledger/goshimmer/client/wallet"
	"github.com/iotaledger/goshimmer/packages/consensus/gof"
	"github.com/iotaledger/goshimmer/packages/jsonmodels"
	"github.com/iotaledger/goshimmer/packages/ledgerstate"
)

type ServersStatus []*wallet.ServerStatus

type Connector interface {
	// ServersStatuses retrieves the connected server status for each client.
	ServersStatuses() ServersStatus
	// ServerStatus retrieves the connected server status.
	ServerStatus(cltIdx int) (status *wallet.ServerStatus, err error)
	// Clients returns list of all clients.
	Clients(...bool) []Client
	// GetClients returns the numOfClt client instances that were used the longest time ago.
	GetClients(numOfClt int) []Client
	// AddClient adds client to WebClients based on provided GoShimmerAPI url.
	AddClient(url string, setters ...client.Option)
	// RemoveClient removes client with the provided url from the WebClients.
	RemoveClient(url string)
	// GetClient returns the client instance that was used the longest time ago.
	GetClient() Client
	// PledgeID returns the node ID that the mana will be pledging to.
	PledgeID() *identity.ID
}

// WebClients is responsible for handling connections via GoShimmerAPI.
type WebClients struct {
	clients []*WebClient
	urls    []string

	// can be used in case we want all mana to be pledge to a specific node
	pledgeID *identity.ID
	// helper variable indicating which clt was recently used, useful for double, triple,... spends
	lastUsed int

	mu sync.Mutex
}

// NewWebClients creates Connector from provided GoShimmerAPI urls.
func NewWebClients(urls []string, setters ...client.Option) *WebClients {
	clients := make([]*WebClient, len(urls))
	for i, url := range urls {
		clients[i] = NewWebClient(url, setters...)
	}

	return &WebClients{
		clients:  clients,
		urls:     urls,
		lastUsed: -1,
	}
}

// ServersStatuses retrieves the connected server status for each client.
func (c *WebClients) ServersStatuses() ServersStatus {
	status := make(ServersStatus, len(c.clients))

	for i := range c.clients {
		status[i], _ = c.ServerStatus(i)
	}
	return status
}

// ServerStatus retrieves the connected server status.
func (c *WebClients) ServerStatus(cltIdx int) (status *wallet.ServerStatus, err error) {
	response, err := c.clients[cltIdx].api.Info()
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
func (c *WebClients) Clients(...bool) []Client {
	clients := make([]Client, len(c.clients))
	for i, c := range c.clients {
		clients[i] = c
	}
	return clients
}

// GetClients returns the numOfClt client instances that were used the longest time ago.
func (c *WebClients) GetClients(numOfClt int) []Client {
	c.mu.Lock()
	defer c.mu.Unlock()

	clts := make([]Client, numOfClt)

	for i := range clts {
		clts[i] = c.getClient()
	}
	return clts
}

// getClient returns the client instance that was used the longest time ago, not protected by mutex.
func (c *WebClients) getClient() Client {
	if c.lastUsed >= len(c.clients)-1 {
		c.lastUsed = 0
	} else {
		c.lastUsed++
	}
	return c.clients[c.lastUsed]
}

// GetClient returns the client instance that was used the longest time ago.
func (c *WebClients) GetClient() Client {
	c.mu.Lock()
	defer c.mu.Unlock()

	return c.getClient()
}

// AddClient adds client to WebClients based on provided GoShimmerAPI url.
func (c *WebClients) AddClient(url string, setters ...client.Option) {
	c.mu.Lock()
	defer c.mu.Unlock()

	clt := NewWebClient(url, setters...)
	c.clients = append(c.clients, clt)
	c.urls = append(c.urls, url)
}

// RemoveClient removes client with the provided url from the WebClients.
func (c *WebClients) RemoveClient(url string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	indexToRemove := -1
	for i, u := range c.urls {
		if u == url {
			indexToRemove = i
			break
		}
	}
	if indexToRemove == -1 {
		return
	}
	c.clients = append(c.clients[:indexToRemove], c.clients[indexToRemove+1:]...)
	c.urls = append(c.urls[:indexToRemove], c.urls[indexToRemove+1:]...)
}

// PledgeID returns the node ID that the mana will be pledging to.
func (c *WebClients) PledgeID() *identity.ID {
	return c.pledgeID
}

// SetPledgeID sets the node ID that the mana will be pledging to.
func (c *WebClients) SetPledgeID(id *identity.ID) {
	c.pledgeID = id
}

type Client interface {
	// Url returns a client API url.
	Url() (cltID string)
	// PostTransaction sends a transaction to the Tangle via a given client.
	PostTransaction(tx *ledgerstate.Transaction) (ledgerstate.TransactionID, error)
	// PostData sends the given data (payload) by creating a message in the backend.
	PostData(data []byte) (msgID string, err error)
	// GetUnspentOutputForAddress gets the first unspent outputs of a given address.
	GetUnspentOutputForAddress(addr ledgerstate.Address) *jsonmodels.WalletOutput
	// GetAddressUnspentOutputs gets the unspent outputs of an address.
	GetAddressUnspentOutputs(address string) (outputIDs []ledgerstate.OutputID, err error)
	// GetTransactionGoF returns the GoF of a given transaction ID.
	GetTransactionGoF(txID string) gof.GradeOfFinality
	// GetOutput gets the output of a given outputID.
	GetOutput(outputID ledgerstate.OutputID) ledgerstate.Output
	// GetOutputGoF gets the first unspent outputs of a given address.
	GetOutputGoF(outputID ledgerstate.OutputID) gof.GradeOfFinality
	// SendFaucetRequest requests funds from the faucet and returns the faucet request message ID.
	SendFaucetRequest(address string) error
	// GetTransactionOutputs returns the outputs the transaction created.
	GetTransactionOutputs(txID string) (outputs ledgerstate.Outputs, err error)
	// GetTransaction gets the transaction.
	GetTransaction(txID string) (resp *jsonmodels.Transaction, err error)
	// GetOutputSolidity checks if the transaction is solid.
	GetOutputSolidity(outID string) (solid bool, err error)
}

// WebClient contains a GoShimmer web API to interact with a node.
type WebClient struct {
	api *client.GoShimmerAPI
	url string
}

// Url returns a client API Url.
func (c *WebClient) Url() string {
	return c.url
}

// NewWebClient creates Connector from provided GoShimmerAPI urls.
func NewWebClient(url string, setters ...client.Option) *WebClient {
	return &WebClient{
		api: client.NewGoShimmerAPI(url, setters...),
		url: url,
	}
}

// SendFaucetRequest requests funds from the faucet and returns the faucet request message ID.
func (c *WebClient) SendFaucetRequest(address string) (err error) {
	_, err = c.api.SendFaucetRequest(address, -1)
	return
}

// PostTransaction sends a transaction to the Tangle via a given client.
func (c *WebClient) PostTransaction(tx *ledgerstate.Transaction) (txID ledgerstate.TransactionID, err error) {
	resp, err := c.api.PostTransaction(tx.Bytes())
	if err != nil {
		return
	}
	txID, err = ledgerstate.TransactionIDFromBase58(resp.TransactionID)
	if err != nil {
		return
	}
	return
}

// PostData sends the given data (payload) by creating a message in the backend.
func (c *WebClient) PostData(data []byte) (msgID string, err error) {
	resp, err := c.api.Data(data)
	if err != nil {
		return
	}

	return resp, nil
}

func (c *WebClient) GetAddressUnspentOutputs(address string) (outputIDs []ledgerstate.OutputID, err error) {
	res, err := c.api.GetAddressUnspentOutputs(address)
	if err != nil {
		return
	}
	outputIDs = getOutputIDsByJSON(res.Outputs)
	return
}

// GetUnspentOutputForAddress gets the first unspent outputs of a given address.
func (c *WebClient) GetUnspentOutputForAddress(addr ledgerstate.Address) *jsonmodels.WalletOutput {
	resp, err := c.api.PostAddressUnspentOutputs([]string{addr.Base58()})
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
func (c *WebClient) GetOutputGoF(outputID ledgerstate.OutputID) gof.GradeOfFinality {
	res, err := c.api.GetOutputMetadata(outputID.Base58())
	if err != nil {
		return gof.None
	}

	return res.GradeOfFinality
}

// GetOutput gets the output of a given outputID.
func (c *WebClient) GetOutput(outputID ledgerstate.OutputID) ledgerstate.Output {
	res, err := c.api.GetOutput(outputID.Base58())
	if err != nil {
		return nil
	}
	output := getOutputByJSON(res)
	return output
}

// GetTransactionGoF returns the GoF of a given transaction ID.
func (c *WebClient) GetTransactionGoF(txID string) gof.GradeOfFinality {
	resp, err := c.api.GetTransactionMetadata(txID)
	if err != nil {
		return gof.None
	}
	return resp.GradeOfFinality
}

// GetTransactionOutputs returns the outputs the transaction created.
func (c *WebClient) GetTransactionOutputs(txID string) (outputs ledgerstate.Outputs, err error) {
	resp, err := c.api.GetTransaction(txID)
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

// GetTransaction gets the transaction.
func (c *WebClient) GetTransaction(txID string) (resp *jsonmodels.Transaction, err error) {
	resp, err = c.api.GetTransaction(txID)
	if err != nil {
		return
	}
	return
}

func (c *WebClient) GetOutputSolidity(outID string) (solid bool, err error) {
	resp, err := c.api.GetOutputMetadata(outID)
	if err != nil {
		return
	}
	solid = resp.Solid
	return
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
