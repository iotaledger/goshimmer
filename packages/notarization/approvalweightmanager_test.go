package notarization

import (
	"github.com/iotaledger/goshimmer/packages/tangle"
	"github.com/stretchr/testify/require"
	"testing"
)

// TestApprovalWeightManager_ProcessMessage tests the whole functionality of the ApprovalWeightManager.
// The scenario can be found in images/approvalweight-processMessage.png.
func TestApprovalWeightManager_ProcessMessage(t *testing.T) {
	processMsgScenario := tangle.ProcessMessageScenario(t)
	defer func(processMsgScenario *tangle.TestScenario, t *testing.T) {
		if err := processMsgScenario.Cleanup(t); err != nil {
			require.NoError(t, err)
		}
	}(processMsgScenario, t)

	for processMsgScenario.HasNext() {
		processMsgScenario.Next(nil)
	}
}
