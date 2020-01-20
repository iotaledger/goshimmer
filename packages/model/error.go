package model

import "errors"

var (
	ErrUnmarshalFailed = errors.New("unmarshal failed")
	ErrMarshalFailed   = errors.New("marshal failed")
)
