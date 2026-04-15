package v1

import (
	openapi_types "github.com/oapi-codegen/runtime/types"

	dofficial "cixing/internal/modules/official/domain"
	readmodelapp "cixing/internal/modules/readmodel/application"
	v1gen "cixing/internal/transport/http/v1/gen"
)

func todayKeywordResponse(day readmodelapp.OfficialHomeDay) v1gen.TodayKeywordResponse {
	return v1gen.TodayKeywordResponse{
		BizDate:              openapi_types.Date{Time: day.BizDate},
		Keyword:              officialKeywordResponse(day.Keyword),
		ParticipantUserCount: int(day.ParticipantUserCount),
	}
}

func meHomeResponse(out *readmodelapp.MeHomeOutput) v1gen.MeHomeResponse {
	items := make([]v1gen.MeHomeCustomKeywordItem, 0, len(out.CustomKeywords))
	for _, item := range out.CustomKeywords {
		respItem := v1gen.MeHomeCustomKeywordItem{
			Id:              openapi_types.UUID(item.ID),
			Text:            item.Text,
			TotalImageCount: int(item.TotalImageCount),
			MyImageCount:    int(item.MyImageCount),
		}
		if item.TargetImageCount != nil {
			v := int(*item.TargetImageCount)
			respItem.TargetImageCount = &v
		}
		if item.CoverImage != nil {
			coverImage := v1gen.ImageRefSquareSmall{
				Id: openapi_types.UUID(item.CoverImage.ID),
			}
			coverImage.Variants.SquareSmall = v1gen.ImageVariant{
				Url:    item.CoverImage.SquareSmall.URL,
				Width:  int(item.CoverImage.SquareSmall.Width),
				Height: int(item.CoverImage.SquareSmall.Height),
			}
			respItem.CoverImage = &coverImage
		}
		items = append(items, respItem)
	}
	return v1gen.MeHomeResponse{
		Nickname:                out.Nickname,
		AvatarUrl:               out.AvatarURL,
		OfficialImageCount:      int(out.OfficialImageCount),
		CustomImageCount:        int(out.CustomImageCount),
		CustomKeywords:          items,
		UnreadNotificationCount: int(out.UnreadNotificationCount),
	}
}

func publicUploadListResponse(out *readmodelapp.PublicUploadListOutput) v1gen.ListPublicUploadsResponse {
	items := make([]v1gen.PublicWorkUploadCard, 0, len(out.Items))
	for _, item := range out.Items {
		items = append(items, publicUploadCardResponse(item))
	}
	return v1gen.ListPublicUploadsResponse{
		Items: items,
		Seed:  out.Seed,
	}
}

func publicUploadCardResponse(item readmodelapp.PublicUploadCardOutput) v1gen.PublicWorkUploadCard {
	resp := v1gen.PublicWorkUploadCard{
		Id:        openapi_types.UUID(item.ID),
		BizDate:   openapi_types.Date{Time: item.BizDate},
		KeywordId: openapi_types.UUID(item.KeywordID),
		CoverImage: v1gen.ImageRefCard4x3{
			Id: openapi_types.UUID(item.CoverImage.ID),
		},
		DisplayText:   item.DisplayText,
		CoverHasAudio: item.CoverHasAudio,
		ImageCount:    int(item.ImageCount),
		CreatedAt:     item.CreatedAt,
	}
	resp.CoverImage.Variants.Card4x3 = v1gen.ImageVariant{
		Url:    item.CoverImage.Card4x3.URL,
		Width:  int(item.CoverImage.Card4x3.Width),
		Height: int(item.CoverImage.Card4x3.Height),
	}
	if item.CoverAudioDuration != nil {
		v := int(*item.CoverAudioDuration)
		resp.CoverAudioDurationMs = &v
	}
	if item.ReactionCounts != nil {
		resp.ReactionCounts = &v1gen.ReactionCounts{
			Inspired:  int(item.ReactionCounts.Inspired),
			Resonated: int(item.ReactionCounts.Resonated),
		}
	}
	if len(item.MyReactions) > 0 {
		reactions := make([]v1gen.ReactionType, 0, len(item.MyReactions))
		for _, reaction := range item.MyReactions {
			reactions = append(reactions, v1gen.ReactionType(reaction))
		}
		resp.MyReactions = &reactions
	}
	return resp
}

func publicUploadDetailResponse(out *readmodelapp.PublicUploadDetailOutput) v1gen.PublicWorkUploadDetail {
	card := publicUploadCardResponse(out.PublicUploadCardOutput)
	return v1gen.PublicWorkUploadDetail{
		Id:                   card.Id,
		BizDate:              card.BizDate,
		KeywordId:            card.KeywordId,
		CoverImage:           card.CoverImage,
		DisplayText:          card.DisplayText,
		CoverHasAudio:        card.CoverHasAudio,
		CoverAudioDurationMs: card.CoverAudioDurationMs,
		ImageCount:           card.ImageCount,
		ReactionCounts:       card.ReactionCounts,
		MyReactions:          card.MyReactions,
		CreatedAt:            card.CreatedAt,
		Images:               workImagesResponse(out.Images),
	}
}

func reviewDatesResponse(out *readmodelapp.ReviewDatesOutput) v1gen.ListMyUploadDatesResponse {
	items := make([]v1gen.MyUploadDateItem, 0, len(out.Items))
	for _, item := range out.Items {
		respItem := v1gen.MyUploadDateItem{
			BizDate: openapi_types.Date{Time: item.BizDate},
			Keyword: officialKeywordResponse(item.Keyword),
		}
		if item.MyUploadCount != nil {
			v := int(*item.MyUploadCount)
			respItem.MyUploadCount = &v
		}
		if item.MyImageCount != nil {
			v := int(*item.MyImageCount)
			respItem.MyImageCount = &v
		}
		items = append(items, respItem)
	}
	return v1gen.ListMyUploadDatesResponse{
		TotalParticipationDays: int(out.TotalParticipationDays),
		TotalImageCount:        int(out.TotalImageCount),
		Items:                  items,
	}
}

func reviewKeywordsResponse(out *readmodelapp.ReviewKeywordCountsOutput) v1gen.ListMyOfficialKeywordUploadCountsResponse {
	items := make([]v1gen.MyOfficialKeywordUploadCountItem, 0, len(out.Items))
	for _, item := range out.Items {
		items = append(items, v1gen.MyOfficialKeywordUploadCountItem{
			Keyword:       officialKeywordResponse(item.Keyword),
			MyUploadCount: int(item.MyUploadCount),
			MyImageCount:  int(item.MyImageCount),
		})
	}
	return v1gen.ListMyOfficialKeywordUploadCountsResponse{Items: items}
}

func reviewUploadListResponse(out *readmodelapp.ReviewUploadListOutput) v1gen.ListReviewMyUploadsResponse {
	items := make([]v1gen.ReviewMyWorkUploadCard, 0, len(out.Items))
	for _, item := range out.Items {
		items = append(items, reviewUploadCardResponse(item))
	}
	return v1gen.ListReviewMyUploadsResponse{Items: items}
}

func reviewUploadCardResponse(item readmodelapp.ReviewUploadCardOutput) v1gen.ReviewMyWorkUploadCard {
	resp := v1gen.ReviewMyWorkUploadCard{
		Id:        openapi_types.UUID(item.ID),
		BizDate:   openapi_types.Date{Time: item.BizDate},
		KeywordId: openapi_types.UUID(item.KeywordID),
		CoverImage: v1gen.ImageRefCard4x3{
			Id: openapi_types.UUID(item.CoverImage.ID),
		},
		DisplayText:   item.DisplayText,
		CoverHasAudio: item.CoverHasAudio,
		ImageCount:    int(item.ImageCount),
		CreatedAt:     item.CreatedAt,
	}
	resp.CoverImage.Variants.Card4x3 = v1gen.ImageVariant{
		Url:    item.CoverImage.Card4x3.URL,
		Width:  int(item.CoverImage.Card4x3.Width),
		Height: int(item.CoverImage.Card4x3.Height),
	}
	if item.CoverAudioDuration != nil {
		v := int(*item.CoverAudioDuration)
		resp.CoverAudioDurationMs = &v
	}
	return resp
}

func reviewUploadDetailResponse(out *readmodelapp.ReviewUploadDetailOutput) v1gen.ReviewMyWorkUploadDetail {
	card := reviewUploadCardResponse(out.ReviewUploadCardOutput)
	return v1gen.ReviewMyWorkUploadDetail{
		Id:                   card.Id,
		BizDate:              card.BizDate,
		KeywordId:            card.KeywordId,
		CoverImage:           card.CoverImage,
		DisplayText:          card.DisplayText,
		CoverHasAudio:        card.CoverHasAudio,
		CoverAudioDurationMs: card.CoverAudioDurationMs,
		ImageCount:           card.ImageCount,
		CreatedAt:            card.CreatedAt,
		Images:               workImagesResponse(out.Images),
	}
}

func workImagesResponse(items []readmodelapp.WorkImageOutput) []v1gen.WorkImage {
	out := make([]v1gen.WorkImage, 0, len(items))
	for _, item := range items {
		resp := v1gen.WorkImage{
			Id: openapi_types.UUID(item.ID),
			Image: v1gen.ImageRefDetailLarge{
				Id: openapi_types.UUID(item.Image.ID),
			},
			DisplayOrder: int(item.DisplayOrder),
			Title:        item.Title,
			Note:         item.Note,
			HasAudio:     item.HasAudio,
			CreatedAt:    item.CreatedAt,
		}
		resp.Image.Variants.DetailLarge = v1gen.ImageVariant{
			Url:    item.Image.DetailLarge.URL,
			Width:  int(item.Image.DetailLarge.Width),
			Height: int(item.Image.DetailLarge.Height),
		}
		if item.AudioDurationMs != nil {
			v := int(*item.AudioDurationMs)
			resp.AudioDurationMs = &v
		}
		resp.AudioPlayUrl = item.AudioPlayURL
		out = append(out, resp)
	}
	return out
}

func officialKeywordResponse(keyword dofficial.OfficialKeyword) v1gen.OfficialKeyword {
	return v1gen.OfficialKeyword{
		Id:           openapi_types.UUID(keyword.ID),
		Text:         keyword.Text,
		Category:     v1gen.KeywordCategory(keyword.Category),
		IsActive:     keyword.IsActive,
		DisplayOrder: int(keyword.DisplayOrder),
	}
}
