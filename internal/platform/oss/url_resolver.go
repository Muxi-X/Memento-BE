package oss

import (
	"net/url"
	"strings"
)

// 目前的展示尺寸元信息
const (
	Card4x3Width       int32 = 900
	Card4x3Height      int32 = 1200
	SquareSmallWidth   int32 = 320
	SquareSmallHeight  int32 = 320
	SquareMediumWidth  int32 = 640
	SquareMediumHeight int32 = 640
)

// URLResolver 负责将 OSS 对象键转换成可访问的 URL，并支持图片样式处理
type URLResolver struct {
	publicBaseURL       string
	placeholderImageURL string
	styles              URLStyleConfig
}

type URLResolverConfig struct {
	PublicBaseURL       string
	PlaceholderImageURL string
	Styles              URLStyleConfig // OSS 样式
}

// OSS 样式表
type URLStyleConfig struct {
	Card4x3      string
	SquareSmall  string
	SquareMedium string
	DetailLarge  string
}

// 给前端的图片变体结构
type ResolvedImageVariant struct {
	URL    string
	Width  int32
	Height int32
}

// 样式名称约定
const (
	defaultCard4x3Style      = "card_4x3"
	defaultSquareSmallStyle  = "square_small"
	defaultSquareMediumStyle = "square_medium"
	defaultDetailLargeStyle  = "detail_large"
)

// 创建 URLResolver
func NewURLResolver(cfg URLResolverConfig) *URLResolver {
	return &URLResolver{
		publicBaseURL:       strings.TrimRight(strings.TrimSpace(cfg.PublicBaseURL), "/"),
		placeholderImageURL: strings.TrimSpace(cfg.PlaceholderImageURL),
		styles: URLStyleConfig{
			Card4x3:      defaultStyleName(cfg.Styles.Card4x3, defaultCard4x3Style),
			SquareSmall:  defaultStyleName(cfg.Styles.SquareSmall, defaultSquareSmallStyle),
			SquareMedium: defaultStyleName(cfg.Styles.SquareMedium, defaultSquareMediumStyle),
			DetailLarge:  defaultStyleName(cfg.Styles.DetailLarge, defaultDetailLargeStyle),
		},
	}
}

// 将 OSS 对象键转换成完整的 URL
func (r *URLResolver) ResolveObjectKey(objectKey string) string {
	if r == nil {
		return ""
	}
	objectKey = strings.TrimLeft(strings.TrimSpace(objectKey), "/")
	if objectKey == "" || r.publicBaseURL == "" {
		return ""
	}
	return r.publicBaseURL + "/" + objectKey
}

// 在 URL 后加 OSS 样式
func (r *URLResolver) ResolveObjectKeyWithStyle(objectKey string, style string) string {
	base := r.ResolveObjectKey(objectKey)
	if base == "" {
		return ""
	}
	style = strings.TrimSpace(style)
	if style == "" {
		return base // 没有样式直接返回原 URL
	}
	return base + "?x-oss-process=style/" + url.QueryEscape(style)
}

// 下面是一些常用样式的快捷方法
func (r *URLResolver) ResolveCard4x3ObjectKey(objectKey string) string {
	if r == nil {
		return ""
	}
	return r.ResolveObjectKeyWithStyle(objectKey, r.styles.Card4x3)
}

func (r *URLResolver) ResolveSquareSmallObjectKey(objectKey string) string {
	if r == nil {
		return ""
	}
	return r.ResolveObjectKeyWithStyle(objectKey, r.styles.SquareSmall)
}

func (r *URLResolver) ResolveSquareMediumObjectKey(objectKey string) string {
	if r == nil {
		return ""
	}
	return r.ResolveObjectKeyWithStyle(objectKey, r.styles.SquareMedium)
}

func (r *URLResolver) ResolveDetailLargeObjectKey(objectKey string) string {
	if r == nil {
		return ""
	}
	return r.ResolveObjectKeyWithStyle(objectKey, r.styles.DetailLarge)
}

// 给前端的图片变体结构，包含 URL 和尺寸信息
func (r *URLResolver) ResolveCard4x3Variant(objectKey string) *ResolvedImageVariant {
	url := r.ResolveCard4x3ObjectKey(objectKey)
	if url == "" {
		return nil
	}
	return &ResolvedImageVariant{
		URL:    url,
		Width:  Card4x3Width,
		Height: Card4x3Height,
	}
}

func (r *URLResolver) ResolveSquareSmallVariant(objectKey string) *ResolvedImageVariant {
	url := r.ResolveSquareSmallObjectKey(objectKey)
	if url == "" {
		return nil
	}
	return &ResolvedImageVariant{
		URL:    url,
		Width:  SquareSmallWidth,
		Height: SquareSmallHeight,
	}
}

func (r *URLResolver) ResolveSquareMediumVariant(objectKey string) *ResolvedImageVariant {
	url := r.ResolveSquareMediumObjectKey(objectKey)
	if url == "" {
		return nil
	}
	return &ResolvedImageVariant{
		URL:    url,
		Width:  SquareMediumWidth,
		Height: SquareMediumHeight,
	}
}

func (r *URLResolver) ResolveDetailLargeVariant(objectKey string, sourceWidth, sourceHeight *int32) *ResolvedImageVariant {
	url := r.ResolveDetailLargeObjectKey(objectKey)
	if url == "" || sourceWidth == nil || sourceHeight == nil || *sourceWidth <= 0 || *sourceHeight <= 0 {
		return nil
	}
	return &ResolvedImageVariant{
		URL:    url,
		Width:  *sourceWidth,
		Height: *sourceHeight,
	}
}

// 给前端的占位符图片 URL，目前未使用？
func (r *URLResolver) PlaceholderImageURL() string {
	if r == nil {
		return ""
	}
	return r.placeholderImageURL
}

// 样式名称处理函数，优先使用配置值，否则使用默认值
func defaultStyleName(value string, fallback string) string {
	value = strings.TrimSpace(value)
	if value != "" {
		return value
	}
	return fallback
}
