package wallet

// ServerStatus defines the information of connected server
type ServerStatus struct {
	ID                string
	Synced            bool
	Version           string
	ManaDecay         float64
	DelegationAddress string
}
