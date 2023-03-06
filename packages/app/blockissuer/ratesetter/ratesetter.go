package ratesetter

import (
	"strings"
	"time"

	"github.com/iotaledger/goshimmer/packages/app/blockissuer/ratesetter/aimd"
	"github.com/iotaledger/goshimmer/packages/app/blockissuer/ratesetter/deficit"
	"github.com/iotaledger/goshimmer/packages/app/blockissuer/ratesetter/disabled"
	"github.com/iotaledger/goshimmer/packages/protocol"
	"github.com/iotaledger/hive.go/crypto/identity"
	"github.com/iotaledger/hive.go/runtime/options"
)

// The rate setter can be in one of three modes: aimd, deficit or disabled.
const (
	AIMDMode ModeType = iota
	DeficitMode
	DisabledMode
)

type ModeType int8

func ParseRateSetterMode(s string) ModeType {
	switch strings.ToLower(s) {
	case "aimd":
		return AIMDMode
	case "disabled":
		return DisabledMode
	default:
		return DeficitMode
	}
}

func (m ModeType) String() string {
	switch m {
	case AIMDMode:
		return "aimd"
	case DisabledMode:
		return "disabled"
	default:
		return "deficit"
	}
}

// New creates a new rate setter instance based on provided options.
func New(localID identity.ID, protocol *protocol.Protocol, opts ...options.Option[Options]) (rateSetter RateSetter) {
	rateSetterOpt := &Options{}
	options.Apply(rateSetterOpt, opts)

	switch rateSetterOpt.mode {
	case AIMDMode:
		rateSetter = aimd.New(protocol, localID,
			aimd.WithPause(rateSetterOpt.pause),
			aimd.WithInitialRate(rateSetterOpt.initial),
			aimd.WithSchedulerRate(rateSetterOpt.schedulerRate),
		)
	case DeficitMode:
		rateSetter = deficit.New(protocol, localID,
			deficit.WithSchedulerRate(rateSetterOpt.schedulerRate),
		)
	default:
		rateSetter = disabled.New()
	}
	return
}

// region RateSetter interface ///////////////////////////////////////////////////////////////////////////////////////////////////

type RateSetter interface {
	Rate() float64
	Estimate() time.Duration
	Shutdown()
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region Options //////////////////////////////////////////////////////////////////////////////////////////////////////

type Options struct {
	mode          ModeType
	pause         time.Duration
	initial       float64
	schedulerRate time.Duration
}

func WithMode(mode ModeType) options.Option[Options] {
	return func(o *Options) {
		o.mode = mode
	}
}

func WithPause(pause time.Duration) options.Option[Options] {
	return func(o *Options) {
		o.pause = pause
	}
}

func WithInitialRate(rate float64) options.Option[Options] {
	return func(o *Options) {
		o.initial = rate
	}
}

func WithSchedulerRate(rate time.Duration) options.Option[Options] {
	return func(o *Options) {
		o.schedulerRate = rate
	}
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
