# Protocol high-level overview (WIP)

## Summary

To orientate the reader, we provide a high-level overview of the protocol, following the natural life cycle of a message. The first module used -while the message is still being created-, is the **Tip Selection** module. Here, the node must choose a certain number (from two to eight) of other messages to reference, meaning that the newly created message will be cryptographically attached to these referenced messages. An honest node must always choose tips uniformly at random from a tip pool, i.e., from a set of still unreferenced messages that satisfy a set of conditions, discussed on INSERT LINK. 

![title](Protocol_overview_own_message.png)

Each node in the network has limited bandwidth, CPU, and memory. In order to avoid any node from being overloaded, the right to write in the Tangle is regulated by the congestion control module. Each time a node creates a message, it will attempt to use the network resources to have its message added to everyone's else Tangle. The congestion control module fairly allocates these resources accordingly to a quantity called **Access Mana**, that acts as a Sybil protection mechanism. The exact manner in which Access Mana is acquired is discussed in INSERT LINK. But -for the purposes of this overview- we can summarize Access Mana as a scarce resource, that makes an effective Sybil protection mechanism. Thus, each node has the right to issue messages at a rate proportional to their Access Mana. This fair rate is not constant (since the utilization of the network may fluctuate), and to correctly set its own individual rate of issuance of messages, each node uses a mechanism called the **Rate Setter**, that makes the average issuance rate of the node converge to the fair rate guaranteed by Access Mana. Nodes which do not use the rate Setter will be punished by the Rate control module, which uses adaptive proof of work to limit the rate an attacker can create messages .Essentially, to issue a message, honest nodes must do a small amount of **Proof of Work**. However, if an attacker begins to issue too many messages and floods the network with messages, the difficulty of the proof of work for that node will increase exponentially. Eventually, the attacker will be incapable of issuing new messages.

Between the Rate Setter and the actual gossip of the message, several steps will take place, but -for the sake of clearness- we ignore these steps for now and return to this subject later. Then, assuming that the message was properly created, it will be sent (FIND A BETTER WORD) to the rest of the network. Since we deal with a large number of nodes, the communication graph cannot be complete. Thus, the network topology will be dictated by the **Neighbor Selection** (aka Auto-Peering) module, described in INSERT LINK. 

![title](Protocol_overview_received_message.png)

We turn our attention now to another point of view: the one of the nodes receiving the new messages. After receiving a message, the node will perform several **syntactical verifications**, that will act like a filter to the messages. Additionally, the message has to be **solidified**, meaning that the node must know all the past cone of the transaction. After this step, the node places all the messages left into an inbox. At a fixed global rate (meaning that all nodes use the same rate), the node uses a **scheduler** to choose a message from the inbox. This scheduler works as a gatekeeper, effectively regulating the use of the most scarce resources of the nodes. Since the scheduler works at a fixed rate, the network cannot be overwhelmed. As discussed in INSERT LINK, the scheduler is designed to ensure the following properties:

1. Consistency: all honest nodes will schedule the same messages
2. Fair access: the nodes' messages will be scheduled at a fair rate according to their Access Mana
3. Utilization: if the inbox is not empty, then some message will be scheduled
4. Bounded latency: network delay of all messages will be bounded
5. Security: the properties above hold even in the presence of an attacker

Only after the scheduler, the messages can be written into the local Tangle. To do that, the nodes perform most of the **semantic validation**, such as the search for ("non mergeable"? how can I say that?) conflicts in the message's past cone or (in the case of value transfers) unlock condition checks. At this point (if the message passes this tests), the message will be **booked** into the **local Tangle** of the node and, in the case of a value transfer, the **ledger state** and two vectors called Access Mana vector (already mentioned in this text) and **Consensus Mana** Vector are updated accordingly. The Consensus Mana is another Sybil protection mechanism, but that, since it is applied to different modules than Access Mana, has the need of a different calculation (for more details on that subject, see INSERT LINK). 

![title](Protocol_overview_booking.png)

After having the message booked, the node is free to **gossip** it, but a crucial step of the protocol is still missing: the **opinion Setter** and the voting protocol, that deal with the most subjective parts of the consensus mechanism (notice that, until now, the protocol has mostly dealt with objective checks). The voting protocol used here is the FPC (or **Fast Probabilistic Consensus**), which is a binary voting protocol which allows a large group of nodes to come to consensus on the value of a single bit. The FPC begins with each node having an initial opinion. 

In each round, nodes randomly choose other nodes to query about their opinions. If the number of responses with the opposite opinion is greater than a certain threshold, the querying node changes its opinion. If a node holds the same opinion for a certain number of rounds, it finalizes on that opinion.

In order to prevent liveness attacks, the threshold for changing opinion is determined by a random number issued by a committee of high Consensus Mana nodes via the **dRNG** application. Without the random threshold, an attacker can lie about their responses to prevent the protocol from terminating.

When selecting which other nodes to query, a node must weight the list of all nodes by Consensus Mana. Thus, high Consensus Mana nodes are queried more often then low Consensus Mana nodes. This makes it difficult for an attacker to manipulate the vote. Unless the attacker controls more than 1/3 of the Consensus Mana in the system, with high probability, we know that FPC has the following properties:

1. Termination: every honest node will finalize on some opinion.
2. Agreement: all honest nodes will finalize on the same opinion.
3. Integrity: if a super majority of nodes -e.g. more than 90% weighted by Consensus Mana-, have the same initial opinion, then FPC will terminate with that value.

![title](Protocol_overview_consensus.png)

For more details, see INSERT LINK.  








say something about strong or weak parents

markers - I don't consider them part of the protocol

approval weight - I don't want to mention it, maybe just point to a link to a file about finality



