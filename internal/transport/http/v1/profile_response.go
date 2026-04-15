package v1

import (
	openapi_types "github.com/oapi-codegen/runtime/types"

	profileapp "cixing/internal/modules/profile/application"
	v1gen "cixing/internal/transport/http/v1/gen"
)

func meSettingsResponse(out profileapp.SettingsOutput) v1gen.MeSettings {
	return v1gen.MeSettings{
		Profile:       meProfileResponse(out.Profile),
		Notifications: meNotificationSettingsResponse(out.Notifications),
	}
}

func meProfileResponse(out profileapp.ProfileOutput) v1gen.MeProfile {
	resp := v1gen.MeProfile{
		AvatarUrl: out.AvatarURL,
		Nickname:  out.Nickname,
	}
	if out.Email != nil {
		email := openapi_types.Email(*out.Email)
		resp.Email = &email
	}
	return resp
}

func meNotificationSettingsResponse(out profileapp.NotificationSettingsOutput) v1gen.MeNotificationSettings {
	return v1gen.MeNotificationSettings{
		ReactionEnabled: out.ReactionEnabled,
	}
}
