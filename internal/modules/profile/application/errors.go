package application

import "errors"

var (
	ErrInvalidInput    = errors.New("invalid profile input")
	ErrInvalidNickname = errors.New("invalid profile nickname")
)
