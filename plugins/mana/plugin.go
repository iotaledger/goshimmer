package mana

// TODO: instantiate Access and Consensus Mana Base Vectors

// TODO: try to load base mana vectors from a storage when starting up + save on shutdown
// TODO: listen to TransactionConfirmed
// TODO: derive TxInfo based on transaction
// TODO: trigger bookMana

// TODO: expose plugin functions to the outside

//GetHighestManaNodes(type, n) [n]NodeIdManaTuple: return the n highest type mana nodes (nodeID,manaValue) in ascending order. Should also update their mana value.
//GetManaMap(type) map[nodeID]manaValue: return type mana perception of the node.
//GetAccessMana(nodeID) mana: access Base Mana Vector of Access Mana, update its values with respect to time, and return the amount of Access Mana (either Effective Base Mana 1, Effective Base Mana 2, or some combination of the two). Trigger ManaUpdated event.
//GetConsensusMana(nodeID) mana: access Base Mana Vector of Consensus Mana, update its values with respect to time, and returns the amount of Consensus Mana (either Effective Base Mana 1, Effective Base Mana 2, or some combination of the two). Trigger ManaUpdated event.
//GetNeighborsMana(type): returns the type mana of the nodes neighbors
//GetAllManaVectors() Obtaining the full mana maps for comparison with the perception of other nodes.
//GetWeightedRandomNodes(n): returns a weighted random selection of n nodes. Consensus Mana is used for the weights.
//Obtaining a list of currently known peers + their mana, sorted. Useful for knowing which high mana nodes are online.
//OverrideMana(nodeID, baseManaVector): Sets the nodes mana to a specific value. Can be useful for debugging, setting faucet mana, initialization, etc.. Triggers ManaUpdated
