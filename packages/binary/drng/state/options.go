package state

type Options struct {
	Committee  *Committee
	Randomness *Randomness
}

type Option func(*Options)

// SetCommittee sets the initial committee
func SetCommittee(c *Committee) Option {
	return func(args *Options) {
		args.Committee = c
	}
}

// SetRandomness sets the initial randomness
func SetRandomness(r *Randomness) Option {
	return func(args *Options) {
		args.Randomness = r
	}
}
