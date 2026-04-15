package v1

import (
	openapi_types "github.com/oapi-codegen/runtime/types"

	customapp "cixing/internal/modules/customkeywords/application"
	v1gen "cixing/internal/transport/http/v1/gen"
)

func customKeywordListResponse(items []customapp.KeywordItemOutput) v1gen.ListCustomKeywordsResponse {
	out := make([]v1gen.CustomKeywordItem, 0, len(items))
	for _, item := range items {
		out = append(out, customKeywordItemResponse(item))
	}
	return v1gen.ListCustomKeywordsResponse{Items: out}
}

func customKeywordItemResponse(item customapp.KeywordItemOutput) v1gen.CustomKeywordItem {
	resp := v1gen.CustomKeywordItem{
		Id:              openapi_types.UUID(item.ID),
		Text:            item.Text,
		IsActive:        item.IsActive,
		TotalImageCount: int(item.TotalImageCount),
		MyImageCount:    int(item.MyImageCount),
		CreatedAt:       item.CreatedAt,
	}
	if item.TargetImageCount != nil {
		v := int(*item.TargetImageCount)
		resp.TargetImageCount = &v
	}
	if item.CoverImage != nil {
		cover := v1gen.ImageRefSquareSmall{Id: openapi_types.UUID(item.CoverImage.ID)}
		cover.Variants.SquareSmall = v1gen.ImageVariant{
			Url:    item.CoverImage.SquareSmall.URL,
			Width:  int(item.CoverImage.SquareSmall.Width),
			Height: int(item.CoverImage.SquareSmall.Height),
		}
		resp.CoverImage = &cover
	}
	return resp
}

func customKeywordCoverResponse(out customapp.SetCoverOutput) v1gen.SetCustomKeywordCoverResponse {
	resp := v1gen.SetCustomKeywordCoverResponse{
		KeywordId:   openapi_types.UUID(out.KeywordID),
		CoverSource: v1gen.CustomKeywordCoverSource(out.CoverSource),
	}
	if out.CoverImage != nil {
		image := v1gen.ImageRefDetailLarge{Id: openapi_types.UUID(out.CoverImage.ID)}
		image.Variants.DetailLarge = v1gen.ImageVariant{
			Url:    out.CoverImage.DetailLarge.URL,
			Width:  int(out.CoverImage.DetailLarge.Width),
			Height: int(out.CoverImage.DetailLarge.Height),
		}
		resp.CoverImage = &image
	}
	return resp
}

func customKeywordImagesResponse(out customapp.GalleryOutput) v1gen.ListCustomKeywordImagesResponse {
	resp := v1gen.ListCustomKeywordImagesResponse{
		CoverSource: v1gen.CustomKeywordCoverSource(out.CoverSource),
		Items:       make([]v1gen.CustomKeywordImageCard, 0, len(out.Items)),
	}
	if out.CoverImage != nil {
		cover := v1gen.ImageRefDetailLarge{Id: openapi_types.UUID(out.CoverImage.ID)}
		cover.Variants.DetailLarge = v1gen.ImageVariant{
			Url:    out.CoverImage.DetailLarge.URL,
			Width:  int(out.CoverImage.DetailLarge.Width),
			Height: int(out.CoverImage.DetailLarge.Height),
		}
		resp.CoverImage = &cover
	}
	for _, item := range out.Items {
		image := v1gen.ImageRefSquareMedium{Id: openapi_types.UUID(item.Image.ID)}
		image.Variants.SquareMedium = v1gen.ImageVariant{
			Url:    item.Image.SquareMedium.URL,
			Width:  int(item.Image.SquareMedium.Width),
			Height: int(item.Image.SquareMedium.Height),
		}
		resp.Items = append(resp.Items, v1gen.CustomKeywordImageCard{
			Id:           openapi_types.UUID(item.ID),
			Image:        image,
			DisplayOrder: int(item.DisplayOrder),
			CreatedAt:    item.CreatedAt,
		})
	}
	return resp
}

func customKeywordImageDetailResponse(out customapp.ImageDetailOutput) v1gen.CustomKeywordImageDetail {
	image := v1gen.ImageRefDetailLarge{Id: openapi_types.UUID(out.Image.ID)}
	image.Variants.DetailLarge = v1gen.ImageVariant{
		Url:    out.Image.DetailLarge.URL,
		Width:  int(out.Image.DetailLarge.Width),
		Height: int(out.Image.DetailLarge.Height),
	}
	resp := v1gen.CustomKeywordImageDetail{
		Id:              openapi_types.UUID(out.ID),
		CustomKeywordId: openapi_types.UUID(out.CustomKeywordID),
		Image:           image,
		DisplayOrder:    int(out.DisplayOrder),
		Title:           out.Title,
		Note:            out.Note,
		HasAudio:        out.HasAudio,
		CreatedAt:       out.CreatedAt,
		AudioPlayUrl:    out.AudioPlayURL,
	}
	if out.AudioDurationMs != nil {
		v := int(*out.AudioDurationMs)
		resp.AudioDurationMs = &v
	}
	return resp
}
