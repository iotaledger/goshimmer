package neighborhood

// outbound selection:
// - configuration
// - run
// - neighborhood data
// - ticker to get Verified Peers
// - select next candidate
// - peering request
// - update neighborhood data (potentially drop)
// - update filter data

// inbound selection:
// - configuration
// - run
// - neighborhood data
// - buffered channel to read Peering requests
// - accept/reject
// - update neighborhood data (potentially drop)
// - update filter data
