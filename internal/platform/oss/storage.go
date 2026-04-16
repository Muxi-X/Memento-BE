package oss

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	alioss "github.com/aliyun/alibabacloud-oss-go-sdk-v2/oss"

	"cixing/internal/config"
	"cixing/internal/shared/common"
)

// 与 OSS 交互的核心,上层业务直接依赖
type Storage struct {
	internal *alioss.Client
	public   *alioss.Client
}

// 将 Clients 包装成 Storage，提供操作 OSS 的接口
func NewStorage(ctx context.Context, cfg config.OSSConfig) (*Storage, error) {
	clients, err := NewClients(cfg)
	if err != nil {
		return nil, err
	}
	return &Storage{
		internal: clients.Internal,
		public:   clients.Public,
	}, nil
}

// 给前端生成预签名 URL，允许上传文件到 OSS
func (s *Storage) PresignPut(ctx context.Context, bucket, key, contentType string, size int64, expires time.Duration) (*common.PresignResult, error) {
	// 基础校验
	if s == nil || s.public == nil {
		return nil, fmt.Errorf("oss: public client not initialized")
	}
	if expires <= 0 {
		expires = 15 * time.Minute
	}
	_ = size

	// 构造 OSS SDK 的预签名请求
	req := &alioss.PutObjectRequest{
		Bucket: &bucket,
		Key:    &key,
	}
	if strings.TrimSpace(contentType) != "" {
		req.ContentType = &contentType
	}

	// 调用 SDK 的预签名方法
	result, err := s.public.Presign(ctx, req, alioss.PresignExpires(expires))
	if err != nil {
		return nil, err
	}

	// 返回项目统一结果
	return &common.PresignResult{
		Method:    result.Method,
		URL:       result.URL,
		Headers:   cloneHeaders(result.SignedHeaders),
		ExpiresAt: result.Expiration,
	}, nil
}

// 给前端生成预签名 URL，允许下载 OSS 上的文件
func (s *Storage) PresignGet(ctx context.Context, bucket, key string, expires time.Duration) (*common.PresignResult, error) {
	// 基础校验
	if s == nil || s.public == nil {
		return nil, fmt.Errorf("oss: public client not initialized")
	}
	if expires <= 0 {
		expires = 10 * time.Minute
	}

	// 调用 SDK 的预签名方法
	result, err := s.public.Presign(ctx, &alioss.GetObjectRequest{
		Bucket: &bucket,
		Key:    &key,
	}, alioss.PresignExpires(expires))
	if err != nil {
		return nil, err
	}

	// 返回项目统一结果
	return &common.PresignResult{
		Method:    result.Method,
		URL:       result.URL,
		Headers:   cloneHeaders(result.SignedHeaders),
		ExpiresAt: result.Expiration,
	}, nil
}

// 后端读取 OSS 对象内容
func (s *Storage) Get(ctx context.Context, bucket, key string) ([]byte, string, error) {
	// 基础校验
	if s == nil || s.internal == nil {
		return nil, "", fmt.Errorf("oss: internal client not initialized")
	}

	// 调用 SDK 的 GetObject 方法
	result, err := s.internal.GetObject(ctx, &alioss.GetObjectRequest{
		Bucket: &bucket,
		Key:    &key,
	})
	if err != nil {
		// 处理 "对象不存在" 的情况，返回统一的 NotFound 错误
		if isNotFound(err) {
			return nil, "", common.ErrNotFound
		}
		return nil, "", err
	}
	defer func() { _ = result.Body.Close() }()

	// 读取对象内容并返回
	body, err := io.ReadAll(result.Body)
	if err != nil {
		return nil, "", err
	}
	return body, deref(result.ContentType), nil
}

// 后端直接上传对象到 OSS
func (s *Storage) Put(ctx context.Context, bucket, key string, body []byte, contentType string) error {
	// 基础校验
	if s == nil || s.internal == nil {
		return fmt.Errorf("oss: internal client not initialized")
	}

	// 构造 PutObjectRequest
	req := &alioss.PutObjectRequest{
		Bucket: &bucket,
		Key:    &key,
		Body:   bytes.NewReader(body),
	}
	if strings.TrimSpace(contentType) != "" {
		req.ContentType = &contentType
	}

	// 调用 SDK 的 PutObject 方法
	_, err := s.internal.PutObject(ctx, req)
	return err
}

// 检查对象是否存在，并获取基本信息（大小、ETag、Content-Type）
func (s *Storage) Head(ctx context.Context, bucket, key string) (exists bool, size int64, etag string, contentType string, err error) {
	// 基础校验
	if s == nil || s.internal == nil {
		return false, 0, "", "", fmt.Errorf("oss: internal client not initialized")
	}

	// 调用 SDK 的 HeadObject 方法
	result, err := s.internal.HeadObject(ctx, &alioss.HeadObjectRequest{
		Bucket: &bucket,
		Key:    &key,
	})
	if err != nil {
		if isNotFound(err) {
			return false, 0, "", "", nil
		}
		return false, 0, "", "", err
	}

	return true, result.ContentLength, deref(result.ETag), deref(result.ContentType), nil
}

// 删除 OSS 上的对象
func (s *Storage) Delete(ctx context.Context, bucket, key string) error {
	if s == nil || s.internal == nil {
		return fmt.Errorf("oss: internal client not initialized")
	}
	_, err := s.internal.DeleteObject(ctx, &alioss.DeleteObjectRequest{
		Bucket: &bucket,
		Key:    &key,
	})
	return err
}

func cloneHeaders(src map[string]string) map[string]string {
	if src == nil {
		return nil
	}
	out := make(map[string]string, len(src))
	for k, v := range src {
		out[k] = v
	}
	return out
}

// *string 转 string 的辅助函数，处理 nil 指针的情况
func deref(v *string) string {
	if v == nil {
		return ""
	}
	return *v
}

// 检查是否是 "对象不存在" 的错误
func isNotFound(err error) bool {
	var svcErr *alioss.ServiceError
	if !errors.As(err, &svcErr) {
		return false
	}

	if svcErr.HttpStatusCode() == http.StatusNotFound {
		return true
	}

	switch strings.TrimSpace(svcErr.ErrorCode()) {
	case "NoSuchKey", "NotFound", "NoSuchBucket":
		return true
	default:
		return false
	}
}
