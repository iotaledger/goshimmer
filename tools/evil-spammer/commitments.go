package main

import (
	"time"

	"github.com/iotaledger/goshimmer/client/evilspammer"
	"github.com/iotaledger/goshimmer/tools/evil-spammer/identity"
)

type CommitmentsSpamParams struct {
	CommitmentType string
	Rate           int
	Duration       time.Duration
	TimeUnit       time.Duration
	NetworkAlias   string
	SpammerAlias   string
	ValidAlias     string
	ForkAfter      int // optional, will be used only with CommitmentType = "fork"
}

func CommitmentsSpam(params *CommitmentsSpamParams) {
	identity.LoadConfig()
	SpamCommitments(*params)
}

func SpamCommitments(params CommitmentsSpamParams) {
	privateKey, urlAPI := identity.LoadIdentity(params.NetworkAlias, params.SpammerAlias)
	_, validAPI := identity.LoadIdentity(params.NetworkAlias, params.ValidAlias)
	options := []evilspammer.Options{
		evilspammer.WithClientURL(urlAPI),
		evilspammer.WithValidClientURL(validAPI),
		evilspammer.WithSpamRate(params.Rate, params.TimeUnit),
		evilspammer.WithSpamDuration(params.Duration),
		evilspammer.WithSpammingFunc(evilspammer.CommitmentsSpammingFunction),
		evilspammer.WithIdentity(params.SpammerAlias, privateKey),
		evilspammer.WithCommitmentType(params.CommitmentType),
		evilspammer.WithForkAfter(params.ForkAfter),
	}
	spammer := evilspammer.NewSpammer(options...)
	spammer.Spam()
}
