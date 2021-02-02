package fcob

import "testing"

func TestFCoB(t *testing.T) {
	//
	// opinion := f.deriveOpinion(targetTxID)

	// A B C
	// For each conflicting transaction
	// Solidification/Arrival time
	// Opinion value
	// LoK

	// if in the conflict set there is at least one (LIKE with LoK > 1) {
	//  opinion.Value = dislike
	//  opinion.LoK = 2
	//  return
	// }

	// anchor := the oldest (in terms of solidificationTime) consumer with LoK == 1 || pending
	// if anchor == nil {
	//  opinion.Value = pending
	//  opinion.LoK = 1
	// return
	// }
	// if targetTx.SolidifcationTime.Before(anchor.SolidifcationTime + 10s)
	//  opinion.Value = dislike
	//  opinion.LoK = 1

	// if it is a unlocking locked whatever, then Lok 1
	// if targetTx.SolidifcationTime.After(anchor.SolidifcationTime + 10s)
	//  opinion.Value = dislike
	//  opinion.LoK = 2

	// }

}
