package conflict

import "github.com/iotaledger/hive.go/ds/types"

type TriggerContext[ID comparable] map[ID]types.Empty

func NewTriggerContext[ID comparable](optIDs ...ID) TriggerContext[ID] {
	t := make(TriggerContext[ID])

	for _, id := range optIDs {
		t.Add(id)
	}

	return t
}

func (t TriggerContext[ID]) Add(id ID) {
	t[id] = types.Void
}

func (t TriggerContext[ID]) Has(id ID) bool {
	_, has := t[id]
	return has
}
