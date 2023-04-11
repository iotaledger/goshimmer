package vote

type MockedPower struct {
	VotePower int
}

func (p MockedPower) Compare(other MockedPower) int {
	if p.VotePower-other.VotePower < 0 {
		return -1
	} else if p.VotePower-other.VotePower > 0 {
		return 1
	} else {
		return 0
	}
}
