package main

import (
	"github.com/iotaledger/goshimmer/packages/core/snapshotcreator"
)

var featureNetwork = snapshotcreator.Options{
	GenesisTokenAmount: 1000000000000000,
	FilePath:           "snapshot.bin",
	PeersSeedBase58: []string{
		"AZKt9NEbNb9TAk5SqVTfj3ANoBzrWLjR5YKxa2BCyi8X", // entrynode
		"BYpRNA5aCuyym8SRFbEATraY4yr9oyuXCsCFVcEM8Fm4", // bootstrap_01
		"5UymiW32h2LM7UqVFf5W1f6iH2DxUqA85RnwP5QgyQYa", // vanilla_01
		"HHPL5wTFjihv7sVHKXYbZkGcDbqq75h1LQntBhKs1saX", // node_01
		"7WAEBePov6Po4kUZFN3h7GNHoddTYTEjhJkmmBPHLW2W", // node_02
		"7xKTSQDtZtiGBAapAh7okHJgnYLq5JJtMUDf2sv1eRrc", // node_03
		"oqSAYKz3v587JG5gRKcnPMnjcG9rVd6jFzJ97pjU5Ms",  // node_04
		"J3Vr2cJ4m85xFGmZa1nda7ZZTWWM9ptYCxrUKXFDAFcc", // node_05
		"3X3ZLueaT6T9mGL8C3YUsDrDqsVYvgbXNsa21jhgdzxi", // faucet_01
	},
	InitialAttestation: "BYpRNA5aCuyym8SRFbEATraY4yr9oyuXCsCFVcEM8Fm4",
}

var dockerNetwork = snapshotcreator.Options{
	GenesisTokenAmount: 1000000000000000,
	FilePath:           "docker-network.snapshot",
	PeersSeedBase58: []string{
		"EYsaGXnUVA9aTYL9FwYEvoQ8d1HCJveQVL7vogu6pqCP", // peer_master
		"5kjiW423d5EtNh943j8gYahyxPdnv9xge8Kuks5tjoYg", // peer_master2
		"CHfU1NUf6ZvUKDQHTG2df53GR7CvuMFtyt7YymJ6DwS3", // faucet
	},
	InitialAttestation: "EYsaGXnUVA9aTYL9FwYEvoQ8d1HCJveQVL7vogu6pqCP",
}

var devnet = snapshotcreator.Options{
	GenesisTokenAmount: 1000000000000000,
	FilePath:           "snapshot.bin",
	PeersSeedBase58: []string{
		"7Yr1tz7atYcbQUv5njuzoC5MiDsMmr3hqaWtAsgJfxxr", // entrynode
		"AuQXPFmRu9nKNtUq3g1RLqVgSmxNrYeogt6uRwqYLGvK", // bootstrap_01
		"D9SPFofAGhA5V9QRDngc1E8qG9bTrnATmpZMdoyRiBoW", // vanilla_01
		"CfkVFzXRjJdshjgPpQAZ4fccZs2SyVPGkTc8LmtnbsT",  // node_01
		"AQfLfcKpvt1nWn916ZGSBy7bRPkjEv5sN7fSZ2rFKoPh", // node_02
		"9GLqh2VaDYUiKGn7kwV2EXsnU6Eiv7AEW73bXhfnX6FD", // node_03
		"C3VeWTBAi12JHXKWTvYCxBRyVya6UpbdziiGZNqgh1sB", // node_04
		"HGiFs4jR74yxDMCN8K1Z16QPdwchXokXzYhBLqKW2ssW", // node_05
		"5heLsHxMRdTewXooaaDFGpAoj5c41ah5wTmpMukjdvi7", // faucet_01
	},
	InitialAttestation: "AuQXPFmRu9nKNtUq3g1RLqVgSmxNrYeogt6uRwqYLGvK",
}
