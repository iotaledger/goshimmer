package state

type Options struct {
	Committee  *Committee
	Randomness *Randomness
}

type option func(*Options)

// SetCommittee sets the initial committee
func SetCommittee(c *Committee) option {
	return func(args *Options) {
		args.Committee = c
	}
}

// SetRandomness sets the initial randomness
func SetRandomness(r *Randomness) option {
	return func(args *Options) {
		args.Randomness = r
	}
}
