# Glossary

---
## CONSENSUS
Agreement on a specific datum or value in distributed multi-agent systems, in the presence of faulty processes.

### BLOCKCHAIN BOTTLENECK
As more transactions are issued, the block rate and size become a bottleneck in the system. It can no longer include all incoming transactions promptly. Attempts to speed up block rates will introduce more orphan blocks (blocks being left behind) and reduce the security of the blockchain.

### NAKAMOTO CONSENSUS
Named after the originator of Bitcoin, Satoshi Nakamoto, Nakamoto consensus describes the replacement of voting/communication between known agents with a cryptographic puzzle (Proof-of-Work). Completing the puzzle determines which agent is the next to act.

### MINING RACES
In PoW-based DLTs, competition between nodes to obtain mining rewards and transaction fees are known as mining races. These are undesirable as they favor more powerful nodes, especially those with highly optimized hardware like ASICs. As such, they block participation by regular or IoT hardware and are harmful for the environment.

### PROOF-OF-WORK
Data which is difficult (costly, time-consuming) to produce but easy for others to verify.

---
## COORDINATOR
A trusted entity that issues milestones to guarantee finality and protect the Tangle against attacks.

### MILESTONES
Milestones are transactions signed and issued by the Coordinator. Their main goal is to help the Tangle to grow healthily and to guarantee finality. When milestones directly or indirectly approve a transaction in the Tangle, nodes mark the state of that transaction and its entire history as confirmed.

---
## TANGLE
An append only message data structure where each message references (at least) two other messages.

### SUBTANGLE
A consistent section of the Tangle (i.e. a subset of messages), such that each included message also includes its referenced messages.

---
## NETWORK LAYER
This layer manages the lower layers of internet communication like TCP. It is the most technical, and in some ways the least interesting. In this layer, the connections between nodes are managed by the autopeering and peer discovery modules and the gossip protocol.

### PEERING
The procedure of discovering and connecting to other network nodes.

### NEIGHBORS
Network nodes that are directly connected and can exchange messages without intermediate nodes.

### NODE
A machine which is part of the IOTA network. Its role is to issue new transactions and to validate existing ones.

### SMALL-WORLD NETWORK
A network in which most nodes can be reached from every other node by a few intermediate steps.

### ECLIPSE ATTACK
A cyber-attack that aims to isolate and attack a specific user, rather than the whole network.

### SPLITTING ATTACKS
An attack in which a malicious node attempts to split the Tangle into two branches. As one of the branches grows, the attacker publishes transactions on the other branch to keep both alive. Splitting attacks attempt to slow down the consensus process or conduct a double spend.

### SYBIL ATTACK
An attempt to gain control over a peer-to-peer network by forging multiple fake identities.

---
## COMMUNICATION LAYER
This layer stores and communicates information. This layer contains the “distributed ledger” or the tangle. The rate control and timestamps are in this layer too.

### MESSAGE
The object that is gossiped between neighbors. All gossiped information is included in a message. The most basic unit of information of the IOTA Protocol. Each message has a type and size and contains data.

### MESSAGE OVERHEAD
The additional information (metadata) that needs to be sent along with the actual information (data). This can contain signatures, voting, heartbeat signals, and anything that is transmitted over the network but is not the transaction itself.

### PARENT
A message approved by another message is called a parent to the latter. A parent can be selected as strong or weak parent. If the past cone of the parent is liked the parent is set as strong parent. If the message is liked but its past cone is disliked it is set as a weak parent. This mechanism is called approval switch.

### PAYLOAD
A field in a message which determines the type. Examples are:
* Value payload (type TransactionType)
* FPC Opinion payload (type StatementType)
* dRNG payload
* Salt declaration payload
* Generic data payload

---
### MANA
The reputation of a node is based on a virtual token called mana. This reputation, working as a Sybil protection mechanism, is important for issuing more transactions (see Module 3) and having a higher influence during the voting process (see Module 5).

#### EPOCH
A time interval that is used for a certain type of consensus mana. At the end of each epoch a snapshot of the state of mana distribution in the network is taken. Since this tool employs the timestamp of messages every node can reach consensus on a epoch's mana distibution eventually.

---
### TRANSACTION
A message with payload of type TransactionType. It contains the information of a transfer of funds.

#### UTXO
Unspent transaction output.

#### ORPHAN
A transaction (or block) that is not referenced by any succeeding transaction (or block). An orphan is not considered confirmed and will not be part of the consensus.

#### REATTACHMENT
Resending a transaction by redoing tip selection and referencing newer tips by redoing PoW.

#### HISTORY
The list of transactions directly or indirectly approved by a given transaction.

#### SOLIDIFICATION TIME
The solidification time is the point at which the entire history of a transaction has been received by a node.

#### FINALITY
The property that once a transaction is completed there is no way to revert or alter it. This is the moment when the parties involved in a transfer can consider the deal done. Finality can be deterministic or probabilistic.

---
### TIP SELECTION
The process of selecting previous messages to be referenced by a new message. These references are where a message attaches to the existing data structure. IOTA only enforces that a message approves (at least) two other messages, but the tip selection strategy is left up to the user (with a good default provided by IOTA).

#### TIP
A message that has not yet been approved.

#### LOCAL MODIFIERS
Custom conditions that nodes can take into account during tip selection. In IOTA, nodes do not necessarily have the same view of the Tangle; various kinds of information only locally available to them can be used to strengthen security.

#### APPROVAL SWITCH
When selecting a message as a parent, we can select from the strong or weak tip pool. This mechanism is called approval switch.

#### APPROVAL WEIGHT
A message gains mana weight, by messages approving it directly or indirectly. However, only strong parents can propagate the mana weight to the past, while weak parents obtain the weight from its weak children but don't propagate it.

---
## APPLICATION LAYER
The IOTA Protocol allows for a host of applications to run on the message tangle. Anybody can design an application, and users can decide which applications to run on their nodes. These applications will all use the communication layer to broadcast and store data.

### CORE APPLICATIONS
Applications that are necessary for the protocol to operate. These include for example:
* The value transfer application
* The distributed random number generator (DRNG for short)
* The Fast Probabilistic Consensus (FPC) protocol

### VALUE TRANSFER APPLICATION
The application which maintains the ledger state.

### FAUCET
A test application issuing funds on request.

---
## MARKERS
A tool that exists only locally and allows performing certain calculations more efficiently. Such as approval weight calculation or the existence of certain messages in the past or future cone of another message.




