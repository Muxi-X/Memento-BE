package application

import "errors"

var (
	ErrInvalidInput             = errors.New("invalid profile input")
	ErrInvalidNickname          = errors.New("invalid profile nickname")
	ErrInvalidAvatarUploadInput = errors.New("invalid avatar upload input")
	ErrAvatarUploadExpired      = errors.New("avatar upload session expired")
)
