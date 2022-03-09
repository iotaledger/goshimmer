---
description: Every network has to deal with its intrinsic limited resources. GoShimmer uses congestion control algorithm to regulate the influx of messages in the network with the goal of maximizing throughput (messages/bytes per second) and minimizing delays.
image: /img/protocol_specification/congestion_control_algorithm_infographic_new.png
keywords:

- node
- congestion control algorithm
- honest node
- message
- access mana
- malicious nde
- scheduling

---

# Congestion Control

Every network has to deal with its limited intrinsic resources in bandwidth and node capabilities (CPU and
storage). In this document, we present a congestion control algorithm to regulate the influx of messages in the
network to maximize throughput (messages/bytes per second) and minimize delays. Furthermore, the
following requirements must be satisfied:

* __Consistency__: If an honest node writes a message, it should be written by all honest nodes within some
  delay bound.
* __Fairness__: Nodes can obtain a share of the available throughput depending on their access Mana. Throughput is
  shared in a way that an attempt to increase the allocation of any node necessarily results in the decrease
  in the allocation of some other node with an equal or smaller allocation (max-min fairness).
* __Security__: Malicious nodes shall be unable to interfere with either of the above requirements.

[![Congestion Control](/img/protocol_specification/congestion_control_algorithm_infographic_new.png)](/img/protocol_specification/congestion_control_algorithm_infographic_new.png)

You can find more information in the following papers:

* [Access Control for Distributed Ledgers in the Internet of Things: A Networking Approach](https://arxiv.org/abs/2005.07778)
  .
* [Secure Access Control for DAG-based Distributed Ledgers](https://arxiv.org/abs/2107.10238).

## Detailed Design

The algorithm has three core components:

* A scheduling algorithm that ensures fair access for all nodes according to their access Mana.
* A TCP-inspired algorithm for decentralized rate setting to utilize the available bandwidth efficiently while
  preventing large delays.
* A buffer management policy to deal with malicious flows.

### Prerequirements

* __Node identity__: The congestion control module requires node accountability. Each message is associated with the node ID of its issuing
  node.

* __Access mana__: The congestion control module knows the access Mana of the network nodes to share the available
  throughput fairly. Without access Mana, the network would be subject to Sybil attacks, which would incentivize actors
  to artificially split (or aggregate) onto multiple identities.

* __Message weight__. The weight of a message is used to prioritize messages over the others, and it is calculated
  based on the type and length of a message.

### Outbox Buffer Management

Once a message has successfully passed the message parser checks, is solid and booked, it is enqueued into the outbox
buffer for scheduling. The outbox is split into several queues, each one corresponding to a different node issuing
messages. The total outbox buffer size is limited, but individual queues do not have a size limit. This section
describes the operations of message enqueuing and dequeuing into and from the outbox buffer.

The enqueuing mechanism includes the following components:

* __Classification__: The mechanism identifies the queue where the message belongs according to the node ID of
  the message issuer.
* __Message enqueuing__: The message is actually enqueued, the queue is sorted by message timestamps in increasing order
  and counters are updated (e.g., counters for the total number of bytes in the queue).
* __Message skipping__. A message is skipped when it is confirmed while still waiting to be scheduled in the
  outbox. A skipped message is removed from the outbox buffer, and it does not decrease the issuing node's deficit.
  It is not gossipped or added to the tip pool as confirmation indicates that the message already has approvers and
  is replicated on enough nodes in the network.
* __Message drop__: In some circumstances, due to network congestion or ongoing attacks, some messages shall be dropped
  to guarantee bounded delays and isolate the attacker's messages. Specifically, a node shall drop messages in the
  following situations:
    - Since the total outbox buffer has a limited size, if the total number of bytes in all queues exceeds the current
      buffer size is smaller than the maximum size. The Mana scaled length of a queue is calculated by dividing the number
      of bytes in the queue by the amount of access Mana of the corresponding node:
    - `Mana-scaled queue size = queue size / node aMana`;

The dequeuing mechanism includes the following components:

* __Queue selection__: A queue is selected according to a round-robin scheduling algorithm. In particular, the 
mechanism uses a modified version of the deficit round-robin (DRR) algorithm.
* __Message dequeuing__. The first (oldest) message of the queue, that satisfies certain conditions is dequeued. A 
  message must satisfy the following conditions:
    * The message has a ready flag assigned. Ready flag is assigned to a message when all of its parents are eligible (parents have been scheduled or confirmed).
    * The message timestamp is not in the future.
* __Scheduler management__: The scheduler counters and pointers are updated.

#### False positive drop

During an attack or congestion, a node may drop a message already scheduled by the rest of the network, causing a _
false positive drop_. This means that the message’s future cone will not be marked as _ready_ as its past cone is not
eligible. This will be resolved when the rest of the network confirms the message, making it eligible and satisfying
the _consistency_ requirement.

#### False negative drop

Another possible problem is that a node schedules a message that the rest of the network drops, causing a _false
negative drop_. The message is gossiped and added to the tip pool. However, it will never accumulate enough approval
weight to be _Confirmed_. Eventually, the node will orphan the message node as
the [Time Since Confirmation check](#tip-pool-and-time-since-confirmation-check)  during tip selection will fail.

### Scheduler

Scheduling is the most critical task in the congestion control component. The scheduling algorithm must guarantee that
an honest node `node` meets the following requirements:

* __Consistency__: The node's messages will not accumulate indefinitely at any node, and so, starvation is avoided.
* __Fairness__: The node's fair share of the network resources are allocated to it according to its access Mana.
* __security__: Malicious nodes sending above their allowed rate will not interrupt a node's throughput requirement.

Although nodes in our setting are capable of more complex and customised behaviour than a typical router in a
packet-switched network, our scheduler must still be lightweight and scalable due to the potentially large number of
nodes requiring differentiated treatment. It is estimated that over 10,000 nodes operate on the Bitcoin network, and
we expect that an even greater number of nodes are likely to be present in the IoT setting. For this reason, we
adopt a scheduler based on [Deficit Round Robin](https://ieeexplore.ieee.org/document/502236) (DRR) (the Linux
implementation of the [FQ-CoDel packet scheduler](https://tools.ietf.org/html/rfc8290), which is based on DRR,
supports anywhere up to 65535 separate queues).

The DRR scans all non-empty queues in sequence. When it selects a non-empty queue, the DDR will increment the queue's
priority counter (_deficit_) by a specific value (_quantum_). Then, the value of the deficit counter is a maximal amount
of bytes that can be sent this turn. If the deficit counter is greater than the weight of the message at the head of the
queue, the DRR can schedule this message, and this weight decrements the value of the counter. In our implementation,
the quantum is proportional to the node's access Mana, and we add a cap on the maximum deficit that a node can achieve
to keep the network latency low. It is also important to mention that the DRR can assign the weight of the message so
that specific messages can be prioritized (low weight) or penalized (large weight); by default, in our mechanism, the
weight is proportional to the message size measured in bytes. The weight of a message is set by the
function `WorkCalculator()`.

:::note

The network manager sets up the desired maximum (fixed) rate `SCHEDULING_RATE` at which it will schedule messages,
computed in weight per second. This implies that every message is scheduled after a delay which is equal to the weight (
size as default) of the latest scheduled message times the parameter
`SCHEDULING_RATE`. This rate mainly depends on the degree of decentralization you desire: a larger rate leads to
higher throughput but will leave behind slower devices that will fall out of sync.

:::

### Rate Setting

If nodes were continuously willing to issue new messages,rate-setting would not be a problem. Nodes could simply operate
at a fixed, assured rate and share the total throughput according to the percentage of access Mana they own. The
scheduling algorithm would ensure that this rate is enforceable, and only misbehaving nodes would experience increasing
delays or dropped messages. However, it is unrealistic to expect all nodes always to have messages to issue. We would
like nodes to better utilize network resources without causing excessive congestion and violating any requirement.

We propose a rate-setting algorithm inspired by TCP — each node employs [additive increase, multiplicative decrease]
(https://https://epubs.siam.org/doi/book/10.1137/1.9781611974225) (AIMD) rules to update their issuance rate in response
to congestion events. In the case of distributed ledgers, all message traffic passes through all nodes, contrary to the
case of traffic typically found in packet-switched networks and other traditional network architectures. Under these
conditions, local congestion at a node is all that is required to indicate congestion elsewhere in the network. This
observation is crucial as it presents an opportunity for a congestion control algorithm based entirely on local traffic.

Our rate-setting algorithm outlines the AIMD rules employed by each node to set their issuance rate. Rate updates for a
node occur each time a new message is scheduled if the node has a non-empty set of its own messages that are not yet
scheduled. The node sets its own local additive-increase variable `localIncrease(node)` based on its access Mana and a
global increase rate parameter `RATE_SETTING_INCREASE`. An appropriate choice of
`RATE_SETTING_INCREASE` ensures a conservative global increase rate that does not cause problems even when many nodes
simultaneously increase their rate.

Nodes wait `RATE_SETTING_PAUSE` seconds after a global multiplicative decrease parameter `RATE_SETTING_DECREASE`, during
which no further updates are made, to allow the reduced rate to take effect and prevent multiple successive decreases.
At each update, the node checks how many of its own messages are in its outbox queue and responds with a multiplicative
decrease if this number is above a threshold,
`backoff(node)`, which is proportional to the node's access Mana. If the number of the node's messages in the outbox is
below the threshold, the node's issuance rate is incremented by its local increase variable, `localIncrease(node)`.

### Tip Pool and Time Since Confirmation Check

When a message is scheduled, it is gossiped to the node's neighbors and, normally, added to the tip pool except in the following situations:

* a confirmed message shall not be added to the tip pool (it shall be skipped by the scheduler)
* a message that has confirmed or scheduled approvers shall not be added to the tip pool
Additionally, strong parents of a message are removed from the tip pool, when the message is added and unused tips are removed from the tip pool after a certain amount of time.

When selecting tips from the tip pool an additional check shall be performed to make sure that the timestamp and a
past cone of a selected message is valid. For a selected tip, the algorithm needs to find a timestamp of the oldest
unconfirmed message in the past cone of the tip (`TS_oum`) . If the difference between current clock time `now` and the
timestamp of the oldest unconfirmed message is greater than a certain threshold (`now - TS_oum > TSC_threshold`), then
the tip cannot be selected and another one needs to be found. The tip shall stay in the tip pool until it is
automatically removed because of its age.

The Time Since Confirmation check solves the mention problem of false negative drops by eventually orphaning messages
that were dropped by the network. 