package media

import "errors"

var (
	ErrInvalidAssetStatusTransition   = errors.New("invalid media asset status transition")
	ErrInvalidVariantStatusTransition = errors.New("invalid media variant status transition")
	ErrInvalidMediaMetadata           = errors.New("invalid media metadata")
)
