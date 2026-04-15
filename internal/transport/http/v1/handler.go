package v1

import (
	authapp "cixing/internal/modules/auth/application"
	customapp "cixing/internal/modules/customkeywords/application"
	officialapp "cixing/internal/modules/official/application"
	profileapp "cixing/internal/modules/profile/application"
	publishingapp "cixing/internal/modules/publishing/application"
	readmodelapp "cixing/internal/modules/readmodel/application"
	socialapp "cixing/internal/modules/social/application"
	v1gen "cixing/internal/transport/http/v1/gen"
)

// service 容器
type Handler struct {
	Signup *authapp.SignupService
	Login  *authapp.LoginService
	Reset  *authapp.PasswordResetService

	OfficialPrompts    *officialapp.PromptService
	PublishingSessions *publishingapp.UploadSessionService
	CustomKeywords     *customapp.Service
	Profile            *profileapp.Service
	ReadModel          *readmodelapp.Service
	SocialReactions    *socialapp.ReactionService
	Notifications      *socialapp.NotificationService
}

var _ v1gen.ServerInterface = (*Handler)(nil)
