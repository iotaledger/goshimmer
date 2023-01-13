package manamodels

import "github.com/pkg/errors"

var (
	// ErrUnknownManaType is returned if mana type could not be identified.
	ErrUnknownManaType = errors.New("unknown mana type")

	// ErrIssuerNotFoundInBaseManaVector is returned if the issuer is not found in the base mana vector.
	ErrIssuerNotFoundInBaseManaVector = errors.New("issuer not present in base mana vector")

	// ErrQueryNotAllowed is returned when the issuer is not synced and mana debug mode is disabled.
	ErrQueryNotAllowed = errors.New("mana query not allowed, issuer is not synced, debug mode disabled")
)
