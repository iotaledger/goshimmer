package main

import (
	"time"

	"github.com/iotaledger/goshimmer/client/evilspammer"
	"github.com/iotaledger/goshimmer/tools/evil-spammer/identity"
)

type CommitmentsSpamParams struct {
	CommitmentType string
	ClientURLs     []string
	ValidIndex     int
	Rate           int
	Duration       time.Duration
	TimeUnit       time.Duration
	NetworkAlias   string
	IdentityAlias  string
	ForkAfter      int // optional, will be used only with CommitmentType = "Fork"
}

func CommitmentsSpam(params *CommitmentsSpamParams) {
	identity.LoadConfig()
	SpamCommitments(*params)
}

func SpamCommitments(params CommitmentsSpamParams) {
	privateKey := identity.LoadIdentity(params.NetworkAlias, params.IdentityAlias)
	options := []evilspammer.Options{
		evilspammer.WithClientURLs(params.ClientURLs),
		evilspammer.WithValidClientURL(params.ClientURLs[params.ValidIndex]),
		evilspammer.WithSpamRate(params.Rate, params.TimeUnit),
		evilspammer.WithSpamDuration(params.Duration),
		evilspammer.WithSpammingFunc(evilspammer.CommitmentsSpammingFunction),
		evilspammer.WithIdentity(params.IdentityAlias, privateKey),
		evilspammer.WithCommitmentType(params.CommitmentType),
		evilspammer.WithForkAfter(params.ForkAfter),
	}
	spammer := evilspammer.NewSpammer(options...)
	spammer.Spam()
}
