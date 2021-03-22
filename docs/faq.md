# FAQ

### What is GoShimmer?
GoShimmer is a research and engineering project from the IOTA Foundation seeking to evaluate Coordicide concepts by implementing them in a node software.

### What kind of confirmation time can I expect?
Since non conflicting transactions aren't even voted up on, they materialize after 2x the average network delay parameter we set. This means that a transaction usually confirms within a time boundary of ~10 seconds.

### Where can I see the state of the Pollen testnet?
You can access the global analysis dashboard [here](http://ressims.iota.cafe:28080/autopeering) showcasing the network graph and active ongoing votes on conflicts.

### How much TPS can GoShimmer sustain?
The transactions per second metric is irrelevant for the current development state of GoShimmer. We are evaluating components from Coordicide and aren't currently interested in squeezing out every little ounce of performance. Meaning, we value simplicity over optimization since the primary goal is to evaluate Coordicide components. Even if we would put out a TPS number, it would simply not reflect an actual metric in a finished production ready node software. 

### How is spamming prevented?
The Coordicide lays out concepts for spam prevention through the means of rate control and such. However, in the current version, GoShimmer relies on PoW to prevent over saturation of the network. Usually doing the PoW for a message will take a couple of seconds on commodity hardware.

### What happens if I issue a double spend?
If you have funds and are simultaneously issuing transactions spending those, then with high certainty your transactions are going to be rejected by the network. This goes even so far, that your funds will be blocked indefinitely (this might change in the future). If you issue a transaction, await the average network delay and then issue the double spend, then the first issued transaction should usually become confirmed and the 2nd one rejected.  

### Who's the target audience for operating a GoShimmer node?
We are mainly interested in individuals helping us out who have a strong IT background, since we simply lack time to help people with things like setting up their nodes, fixing their NAT configs, teaching them how to use Linux and so on. People interested in trying out the bleeding edge of IOTA development and providing meaningful feedback or problem reporting (in form of issues) are welcome. Again, our primary focus is on testing out Coordicide components rather than giving people of any knowledge-level the easiest way to operate a node.