package headers

import (
	"github.com/cockroachdb/errors"

	"github.com/iotaledger/goshimmer/packages/core/diskutil"
)

type Headers struct {
	*Settings
	*Commitments
}

func New(disk *diskutil.DiskUtil) (newHeaderStorage *Headers, err error) {
	newHeaderStorage = &Headers{
		Settings: NewSettings(disk.Path("settings.bin")),
	}
	if newHeaderStorage.Commitments, err = NewCommitments(disk.Path("commitments.bin")); err != nil {
		return nil, errors.Errorf("failed to create commitments storage: %w", err)
	}

	return newHeaderStorage, nil
}

func (c *Headers) Shutdown() (err error) {
	return c.Commitments.Close()
}
