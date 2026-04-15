package domain

import "errors"

var (
	ErrInvalidState    = errors.New("publishing: invalid state")
	ErrExpired         = errors.New("publishing: expired")
	ErrInvalidContext  = errors.New("publishing: invalid context")
	ErrKeywordMismatch = errors.New("publishing: keyword mismatch")
	ErrInvalidAsset    = errors.New("publishing: invalid asset")
)
