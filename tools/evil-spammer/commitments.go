package main

import (
	"time"

	"github.com/iotaledger/goshimmer/client/evilspammer"
	"github.com/iotaledger/goshimmer/tools/evil-spammer/identity"
)

type CommitmentsSpamParams struct {
	CommitmentType string
	ClientURLs     []string
	Rate           int
	Duration       time.Duration
	TimeUnit       time.Duration
	NetworkAlias   string
	IdentityAlias  string
}

func CommitmentsSpam(params *CommitmentsSpamParams) {
	identity.LoadConfig()
	SpamCommitments(params.Rate, params.TimeUnit, params.Duration, params.NetworkAlias, params.IdentityAlias, params.CommitmentType)
}

func SpamCommitments(rate int, timeUnit, duration time.Duration, networkAlias, identityAlias, commitmentType string) {
	privateKey := identity.LoadIdentity(networkAlias, identityAlias)
	options := []evilspammer.Options{
		evilspammer.WithSpamRate(rate, timeUnit),
		evilspammer.WithSpamDuration(duration),
		evilspammer.WithSpammingFunc(evilspammer.CommitmentsSpammingFunction),
		evilspammer.WithIdentity(identityAlias, privateKey),
		evilspammer.WithCommitmentType(commitmentType),
	}
	spammer := evilspammer.NewSpammer(options...)
	spammer.Spam()
}
