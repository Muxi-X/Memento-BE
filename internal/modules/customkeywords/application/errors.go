package application

import "errors"

var (
	ErrInvalidInput            = errors.New("customkeywords: invalid input")
	ErrInvalidKeywordText      = errors.New("customkeywords: invalid keyword text")
	ErrInvalidTargetImageCount = errors.New("customkeywords: invalid target image count")
)
