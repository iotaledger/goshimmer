# List of components

### [Tangle and message layout](./protocol_specification/tangle.md)

The Tangle provides the data structure that maps the relationship of the messages. Messages are build with the concept of payloads which allows for a great number of applications while also improving data processing efficiency. 

### Node identities and [Mana](./mana.md)

Nodes receive rewards for being good actors in a system that secures the network and provides Sybil protection. 

### [Autopeering](./protocol_specification/autopeering.md)

A mechanism for automatically finding neighbors to build a robust ever-changing network.
 
### Timestamps

A declared time variable for messages that allows message ordering, rate control and identifies malicious behavior. 

### Tip selection

Faster, more secure and simpler. The new tip selection is not part of consensus anymore, but it allows us to reach finality within seconds.

### Rate and [congestion control](./protocol_specification/congestion_control.md)

Keeps spammers from harming the network and allocates bandwidth during congested times. 

### [Consensus](./protocol_specification/consensus_mechanism.md)

Decentralized random number generator to provide security, fast voting algorithms through FPC and FCoB to provide quick conflict resolutions and effective finality criteria through approval weight. 

### [Ledger state](./protocol_specification/ledgerstate.md)

The Unspent Transaction Output (UTXO) model together with the Parallel Reality Ledger State enables nodes to efficiently keep track and update all the balances while managing potential conflicts.

### [Advanced Outputs](./protocol_specification/advanced_outputs.md) (Experimental)



## Additional modules:

### [Markers](./markers.md)

The **marker** tool is a tool to efficiently estimate the approval weight of a message and that reduces the portion of the Tangle that needs to be traversed.