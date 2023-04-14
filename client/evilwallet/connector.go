package evilwallet

import (
	"sync"
	"time"

	"github.com/iotaledger/goshimmer/client"
	"github.com/iotaledger/goshimmer/client/wallet"
	"github.com/iotaledger/goshimmer/packages/app/jsonmodels"
	"github.com/iotaledger/goshimmer/packages/core/confirmation"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/ledger/utxo"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/ledger/vm/devnetvm"
	"github.com/iotaledger/goshimmer/packages/protocol/models"
	"github.com/iotaledger/hive.go/crypto/identity"
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
	URL() (cltID string)
	// GetRateSetterEstimate returns a rate setter estimate.
	GetRateSetterEstimate() (estimate time.Duration, err error)
	// SleepRateSetterEstimate sleeps for rate setter estimate.
	SleepRateSetterEstimate() (err error)
	// PostTransaction sends a transaction to the Tangle via a given client.
	PostTransaction(tx *devnetvm.Transaction) (utxo.TransactionID, models.BlockID, error)
	// PostData sends the given data (payload) by creating a block in the backend.
	PostData(data []byte) (blkID string, err error)
	// GetUnspentOutputForAddress gets the first unspent outputs of a given address.
	GetUnspentOutputForAddress(addr devnetvm.Address) *jsonmodels.WalletOutput
	// GetAddressUnspentOutputs gets the unspent outputs of an address.
	GetAddressUnspentOutputs(address string) (outputIDs []utxo.OutputID, err error)
	// GetTransactionConfirmationState returns the ConfirmationState of a given transaction ID.
	GetTransactionConfirmationState(txID string) confirmation.State
	// GetOutput gets the output of a given outputID.
	GetOutput(outputID utxo.OutputID) devnetvm.Output
	// GetOutputConfirmationState gets the first unspent outputs of a given address.
	GetOutputConfirmationState(outputID utxo.OutputID) confirmation.State
	// BroadcastFaucetRequest requests funds from the faucet and returns the faucet request block ID.
	BroadcastFaucetRequest(address string, powTarget int) error
	// GetTransactionOutputs returns the outputs the transaction created.
	GetTransactionOutputs(txID string) (outputs devnetvm.Outputs, err error)
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

// URL returns a client API Url.
func (c *WebClient) URL() string {
	return c.url
}

// NewWebClient creates Connector from provided GoShimmerAPI urls.
func NewWebClient(url string, setters ...client.Option) *WebClient {
	return &WebClient{
		api: client.NewGoShimmerAPI(url, setters...),
		url: url,
	}
}

// GetRateSetterEstimate returns a rate setter estimate.
func (c *WebClient) GetRateSetterEstimate() (estimate time.Duration, err error) {
	response, err := c.api.RateSetter()
	if err != nil {
		return time.Duration(0), err
	}
	return response.Estimate, nil
}

// SleepRateSetterEstimate returns a rate setter estimate.
func (c *WebClient) SleepRateSetterEstimate() (err error) {
	err = c.api.SleepRateSetterEstimate()
	if err != nil {
		return err
	}
	return nil
}

// BroadcastFaucetRequest requests funds from the faucet and returns the faucet request block ID.
func (c *WebClient) BroadcastFaucetRequest(address string, powTarget int) (err error) {
	_, err = c.api.BroadcastFaucetRequest(address, powTarget)
	return
}

// PostTransaction sends a transaction to the Tangle via a given client.
func (c *WebClient) PostTransaction(tx *devnetvm.Transaction) (txID utxo.TransactionID, blockID models.BlockID, err error) {
	txBytes, err := tx.Bytes()
	if err != nil {
		return
	}

	resp, err := c.api.PostTransaction(txBytes)
	if err != nil {
		return
	}
	err = txID.FromBase58(resp.TransactionID)
	if err != nil {
		return
	}
	err = blockID.FromBase58(resp.BlockID)
	if err != nil {
		return
	}
	return
}

// PostData sends the given data (payload) by creating a block in the backend.
func (c *WebClient) PostData(data []byte) (blkID string, err error) {
	resp, err := c.api.Data(data)
	if err != nil {
		return
	}

	return resp, nil
}

func (c *WebClient) GetAddressUnspentOutputs(address string) (outputIDs []utxo.OutputID, err error) {
	res, err := c.api.GetAddressOutputs(address)
	if err != nil {
		return
	}
	outputIDs = getOutputIDsByJSON(res.UnspentOutputs)
	return
}

// GetUnspentOutputForAddress gets the first unspent outputs of a given address.
func (c *WebClient) GetUnspentOutputForAddress(addr devnetvm.Address) *jsonmodels.WalletOutput {
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

// GetOutputConfirmationState gets the first unspent outputs of a given address.
func (c *WebClient) GetOutputConfirmationState(outputID utxo.OutputID) confirmation.State {
	res, err := c.api.GetOutputMetadata(outputID.Base58())
	if err != nil {
		return confirmation.Pending
	}

	return res.ConfirmationState
}

// GetOutput gets the output of a given outputID.
func (c *WebClient) GetOutput(outputID utxo.OutputID) devnetvm.Output {
	res, err := c.api.GetOutput(outputID.Base58())
	if err != nil {
		return nil
	}
	output := getOutputByJSON(res)
	return output
}

// GetTransactionConfirmationState returns the ConfirmationState of a given transaction ID.
func (c *WebClient) GetTransactionConfirmationState(txID string) confirmation.State {
	resp, err := c.api.GetTransactionMetadata(txID)
	if err != nil {
		return confirmation.Pending
	}
	return resp.ConfirmationState
}

// GetTransactionOutputs returns the outputs the transaction created.
func (c *WebClient) GetTransactionOutputs(txID string) (outputs devnetvm.Outputs, err error) {
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
	solid = resp.OutputID.Base58 != ""
	return
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
