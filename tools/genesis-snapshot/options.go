package main

import (
	"github.com/iotaledger/goshimmer/packages/core/snapshotcreator"
	"github.com/iotaledger/goshimmer/packages/protocol"
	"github.com/iotaledger/goshimmer/packages/protocol/ledger/vm/devnetvm"
	"github.com/iotaledger/hive.go/core/generics/options"
)

var BaseOptions = []options.Option[snapshotcreator.Options]{
	snapshotcreator.WithDatabaseVersion(protocol.DatabaseVersion),
	snapshotcreator.WithVM(new(devnetvm.VM)),
	// 7R1itJx5hVuo9w9hjg5cwKFmek4HMSoBDgJZN8hKGxih
	snapshotcreator.WithGenesisSeed([]byte{
		95, 76, 224, 164, 168, 80, 141, 174, 133, 77, 153, 100, 4, 202, 113, 104,
		71, 130, 88, 200, 46, 56, 243, 121, 216, 236, 70, 146, 234, 158, 206, 230,
	}),
	snapshotcreator.WithGenesisTokenAmount(1000000000000000),
	snapshotcreator.WithFilePath("snapshot.bin"),
}

var FeatureNetwork = []options.Option[snapshotcreator.Options]{
	snapshotcreator.WithGenesisTokenAmount(1000000000000000),
	snapshotcreator.WithFilePath("snapshot.bin"),
	snapshotcreator.WithPeersSeedBase58([]string{
		"AZKt9NEbNb9TAk5SqVTfj3ANoBzrWLjR5YKxa2BCyi8X", // entry node
		"BYpRNA5aCuyym8SRFbEATraY4yr9oyuXCsCFVcEM8Fm4", // bootstrap_01
		"5UymiW32h2LM7UqVFf5W1f6iH2DxUqA85RnwP5QgyQYa", // vanilla_01
		"HHPL5wTFjihv7sVHKXYbZkGcDbqq75h1LQntBhKs1saX", // node_01
		"7WAEBePov6Po4kUZFN3h7GNHoddTYTEjhJkmmBPHLW2W", // node_02
		"7xKTSQDtZtiGBAapAh7okHJgnYLq5JJtMUDf2sv1eRrc", // node_03
		"oqSAYKz3v587JG5gRKcnPMnjcG9rVd6jFzJ97pjU5Ms",  // node_04
		"J3Vr2cJ4m85xFGmZa1nda7ZZTWWM9ptYCxrUKXFDAFcc", // node_05
		"3X3ZLueaT6T9mGL8C3YUsDrDqsVYvgbXNsa21jhgdzxi", // faucet_01
	}),
	snapshotcreator.WithInitialAttestationsBase58([]string{"AZKt9NEbNb9TAk5SqVTfj3ANoBzrWLjR5YKxa2BCyi8X"}),
}

var DockerNetwork = []options.Option[snapshotcreator.Options]{
	snapshotcreator.WithGenesisTokenAmount(1000000000000000),
	snapshotcreator.WithFilePath("docker-network.snapshot"),
	snapshotcreator.WithPeersSeedBase58([]string{
		"EYsaGXnUVA9aTYL9FwYEvoQ8d1HCJveQVL7vogu6pqCP", // peer_master
		"5kjiW423d5EtNh943j8gYahyxPdnv9xge8Kuks5tjoYg", // peer_master2
		"CHfU1NUf6ZvUKDQHTG2df53GR7CvuMFtyt7YymJ6DwS3", // faucet
	}),
	snapshotcreator.WithInitialAttestationsBase58([]string{"EYsaGXnUVA9aTYL9FwYEvoQ8d1HCJveQVL7vogu6pqCP"}),
}

var Devnet = []options.Option[snapshotcreator.Options]{
	snapshotcreator.WithGenesisTokenAmount(1000000000000000),
	snapshotcreator.WithFilePath("snapshot.bin"),
	snapshotcreator.WithPeersSeedBase58([]string{
		"7Yr1tz7atYcbQUv5njuzoC5MiDsMmr3hqaWtAsgJfxxr", // entrynode
		"AuQXPFmRu9nKNtUq3g1RLqVgSmxNrYeogt6uRwqYLGvK", // bootstrap_01
		"D9SPFofAGhA5V9QRDngc1E8qG9bTrnATmpZMdoyRiBoW", // vanilla_01
		"CfkVFzXRjJdshjgPpQAZ4fccZs2SyVPGkTc8LmtnbsT",  // node_01
		"AQfLfcKpvt1nWn916ZGSBy7bRPkjEv5sN7fSZ2rFKoPh", // node_02
		"9GLqh2VaDYUiKGn7kwV2EXsnU6Eiv7AEW73bXhfnX6FD", // node_03
		"C3VeWTBAi12JHXKWTvYCxBRyVya6UpbdziiGZNqgh1sB", // node_04
		"HGiFs4jR74yxDMCN8K1Z16QPdwchXokXzYhBLqKW2ssW", // node_05
		"5heLsHxMRdTewXooaaDFGpAoj5c41ah5wTmpMukjdvi7", // faucet_01
	}),
	snapshotcreator.WithInitialAttestationsBase58([]string{"AuQXPFmRu9nKNtUq3g1RLqVgSmxNrYeogt6uRwqYLGvK"}),
}
