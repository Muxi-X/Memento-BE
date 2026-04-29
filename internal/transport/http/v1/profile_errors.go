package v1

import (
	"errors"

	"github.com/gin-gonic/gin"

	dmedia "cixing/internal/modules/media/domain"
	profileapp "cixing/internal/modules/profile/application"
	"cixing/internal/shared/common"
	"cixing/internal/transport/http/server/response"
)

func writeProfileError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, profileapp.ErrInvalidNickname):
		writeFieldValidation(c, "nickname", "length", "must be between 1 and 40 characters")
	case errors.Is(err, profileapp.ErrInvalidInput):
		writeAppError(c, response.Validation, "profile.invalid_input", "invalid profile request", nil)
	case errors.Is(err, common.ErrNotFound):
		writeAppError(c, response.NotFound, "profile.not_found", "not found", nil)
	default:
		writeInternal(c, err)
	}
}

func writeAvatarUploadError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, profileapp.ErrInvalidAvatarUploadInput),
		errors.Is(err, dmedia.ErrInvalidMediaMetadata):
		writeAppError(c, response.Validation, "profile.avatar_invalid_input", "invalid avatar upload request", nil)
	case errors.Is(err, profileapp.ErrAvatarUploadExpired):
		writeAppError(c, response.Conflict, "profile.avatar_session_expired", "avatar upload session expired", nil)
	case errors.Is(err, common.ErrConflict),
		errors.Is(err, dmedia.ErrInvalidAssetStatusTransition):
		writeAppError(c, response.Conflict, "profile.avatar_conflict", "avatar upload conflict", nil)
	case errors.Is(err, common.ErrNotFound):
		writeAppError(c, response.NotFound, "profile.not_found", "not found", nil)
	default:
		writeInternal(c, err)
	}
}
