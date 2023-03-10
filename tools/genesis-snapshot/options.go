package main

import (
	"time"

	"github.com/iotaledger/goshimmer/packages/core/snapshotcreator"
	"github.com/iotaledger/goshimmer/packages/protocol"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/ledger/utxoledger"
	"github.com/iotaledger/hive.go/runtime/options"
)

var BaseOptions = []options.Option[snapshotcreator.Options]{
	snapshotcreator.WithDatabaseVersion(protocol.DatabaseVersion),
	snapshotcreator.WithLedgerProvider(utxoledger.NewProvider()),
	// 7R1itJx5hVuo9w9hjg5cwKFmek4HMSoBDgJZN8hKGxih
	snapshotcreator.WithGenesisSeed([]byte{
		95, 76, 224, 164, 168, 80, 141, 174, 133, 77, 153, 100, 4, 202, 113, 104,
		71, 130, 88, 200, 46, 56, 243, 121, 216, 236, 70, 146, 234, 158, 206, 230,
	}),
	snapshotcreator.WithGenesisTokenAmount(1000000000000000),
	snapshotcreator.WithGenesisUnixTime(time.Now().Unix()),
	snapshotcreator.WithSlotDuration(10),
	snapshotcreator.WithFilePath("snapshot.bin"),
}

var FeatureNetwork = []options.Option[snapshotcreator.Options]{
	snapshotcreator.WithGenesisTokenAmount(1000000000000000),
	snapshotcreator.WithFilePath("feature.snapshot"),
	snapshotcreator.WithPeersPublicKeysBase58([]string{
		"BYpRNA5aCuyym8SRFbEATraY4yr9oyuXCsCFVcEM8Fm4", // bootstrap_01
		"3X3ZLueaT6T9mGL8C3YUsDrDqsVYvgbXNsa21jhgdzxi", // faucet_01
		"5UymiW32h2LM7UqVFf5W1f6iH2DxUqA85RnwP5QgyQYa", // vanilla_01
		"HHPL5wTFjihv7sVHKXYbZkGcDbqq75h1LQntBhKs1saX", // node_01
		"7WAEBePov6Po4kUZFN3h7GNHoddTYTEjhJkmmBPHLW2W", // node_02
		"7xKTSQDtZtiGBAapAh7okHJgnYLq5JJtMUDf2sv1eRrc", // node_03
		"oqSAYKz3v587JG5gRKcnPMnjcG9rVd6jFzJ97pjU5Ms",  // node_04
		"J3Vr2cJ4m85xFGmZa1nda7ZZTWWM9ptYCxrUKXFDAFcc", // node_05
	}),
	snapshotcreator.WithInitialAttestationsBase58([]string{"BYpRNA5aCuyym8SRFbEATraY4yr9oyuXCsCFVcEM8Fm4"}),
}

var DockerNetwork = []options.Option[snapshotcreator.Options]{
	snapshotcreator.WithGenesisTokenAmount(1000000000000000),
	snapshotcreator.WithFilePath("docker-network.snapshot"),
	snapshotcreator.WithPeersPublicKeysBase58([]string{
		"EYsaGXnUVA9aTYL9FwYEvoQ8d1HCJveQVL7vogu6pqCP", // peer_master
		"5kjiW423d5EtNh943j8gYahyxPdnv9xge8Kuks5tjoYg", // peer_master2
		"CHfU1NUf6ZvUKDQHTG2df53GR7CvuMFtyt7YymJ6DwS3", // faucet
	}),
	snapshotcreator.WithInitialAttestationsBase58([]string{"EYsaGXnUVA9aTYL9FwYEvoQ8d1HCJveQVL7vogu6pqCP"}),
}

var Devnet = []options.Option[snapshotcreator.Options]{
	snapshotcreator.WithGenesisTokenAmount(1000000000000000),
	snapshotcreator.WithFilePath("snapshot.bin"),
	snapshotcreator.WithPeersPublicKeysBase58([]string{
		"Gm7W191NDnqyF7KJycZqK7V6ENLwqxTwoKQN4SmpkB24", // bootstrap_01
		"9DB3j9cWYSuEEtkvanrzqkzCQMdH1FGv3TawJdVbDxkd", // vanilla_01
		"AheLpbhRs1XZsRF8t8VBwuyQh9mqPHXQvthV5rsHytDG", // node_01
		"FZ28bSTidszUBn8TTCAT9X1nVMwFNnoYBmZ1xfafez2z", // node_02
		"GT3UxryW4rA9RN9ojnMGmZgE2wP7psagQxgVdA4B9L1P", // node_03
		"4pB5boPvvk2o5MbMySDhqsmC2CtUdXyotPPEpb7YQPD7", // node_04
		"64wCsTZpmKjRVHtBKXiFojw7uw3GszumfvC4kHdWsHga", // node_05
		"7DJYaCCnq9bPW2tnwC3BUEDMs6PLDC73NShduZzE4r9k", // faucet_01
	}),
	snapshotcreator.WithInitialAttestationsBase58([]string{"AuQXPFmRu9nKNtUq3g1RLqVgSmxNrYeogt6uRwqYLGvK"}),
}
