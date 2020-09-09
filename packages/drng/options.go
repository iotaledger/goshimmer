package drng

// Options define state options of a DRNG.
type Options struct {
	// The initial committee of the DRNG.
	Committee *Committee
	// The initial randomness of the DRNG.
	Randomness *Randomness
}

// Option is a function which sets the given option.
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
