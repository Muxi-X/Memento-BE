package v1

import (
	openapi_types "github.com/oapi-codegen/runtime/types"

	socialapp "cixing/internal/modules/social/application"
	v1gen "cixing/internal/transport/http/v1/gen"
)

func notificationsResponse(out socialapp.ListNotificationsOutput) v1gen.ListMyNotificationsResponse {
	items := make([]v1gen.Notification, 0, len(out.Items))
	for _, item := range out.Items {
		resp := v1gen.Notification{
			Id:        openapi_types.UUID(item.ID),
			Type:      v1gen.NotificationType(item.Type.String()),
			UploadId:  openapi_types.UUID(item.UploadID),
			CreatedAt: item.CreatedAt,
			ReadAt:    item.ReadAt,
		}
		if item.ReactionType != nil {
			reactionType := v1gen.ReactionType(item.ReactionType.String())
			resp.ReactionType = &reactionType
		}
		resp.ActorAvatarUrl = item.ActorAvatarURL
		if item.CoverImage != nil {
			resp.CoverImage = &v1gen.NotificationCoverImage{
				SquareSmall: v1gen.ImageVariant{
					Url:    item.CoverImage.SquareSmall.URL,
					Width:  int(item.CoverImage.SquareSmall.Width),
					Height: int(item.CoverImage.SquareSmall.Height),
				},
			}
		}
		items = append(items, resp)
	}
	return v1gen.ListMyNotificationsResponse{Items: items}
}
