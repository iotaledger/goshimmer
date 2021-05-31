# List of components

### Node identities and Mana

Nodes receive rewards for being good actors in a system that secures the network and provides Sybil protection. 

For more details see the [Goshimmer mana specs](./mana.md).

### Message layout

The concept of payloads creates infinite possibilities while also improving data processing efficiency. 

### Autopeering

A mechanism for automatically finding neighbors to build a robust ever-changing network.
 
### Timestamps

A declared time variable for messages that allows message ordering, rate control and identifies malicious behavior. 

### Tip selection

Faster, more secure and simpler. The new tip selection is not part of consensus anymore, but it allows us to reach finality within seconds.

### Rate and congestion control

Keeps spammers from harming the network and allocates bandwidth during congested times. 

### Ledger state

The Unspent Transaction Output (UTXO) model together with the Parallel Reality Ledger State enables nodes to efficiently keep track and update all the balances while managing potential conflicts.

### Consensus

Decentralized random number generator to provide security, fast voting algorithms through FPC and FCoB to provide quick conflict resolutions and effective finality criteria through approval weight. 


## Additional modules:

### Markers

The **marker** tool is a tool to efficiently estimate the approval weight of a message and that reduces the portion of the Tangle that needs to be traversed.

For more details see the [Goshimmer marker specs](./markers.md).
