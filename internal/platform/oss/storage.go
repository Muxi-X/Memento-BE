package oss

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/qiniu/go-sdk/v7/auth"
	"github.com/qiniu/go-sdk/v7/storage"

	"cixing/internal/config"
	"cixing/internal/shared/common"
)

// Storage 封装七牛云 Kodo 的对象操作。
// 实现 common.ObjectStorage 接口，是上层业务的统一入口。
type Storage struct {
	client *Client
}

// NewStorage 根据配置创建 Storage。
func NewStorage(cfg config.OSSConfig) (*Storage, error) {
	client, err := NewClient(cfg)
	if err != nil {
		return nil, err
	}
	return &Storage{client: client}, nil
}

// PresignPut 生成上传凭证，前端凭此凭证直接上传文件到七牛云。
// 七牛云采用上传凭证（uploadToken）而非预签名 PUT URL。
func (s *Storage) PresignPut(ctx context.Context, bucket, key, contentType string, size int64, expires time.Duration) (*common.PresignResult, error) {
	if s == nil || s.client == nil {
		return nil, fmt.Errorf("oss: client not initialized")
	}
	if expires <= 0 {
		expires = 15 * time.Minute
	}

	putPolicy := storage.PutPolicy{
		Scope:   s.client.Bucket + ":" + key,
		Expires: uint64(expires / time.Second),
	}
	upToken := putPolicy.UploadToken(s.client.Mac)

	// 获取上传域名
	uploadHost := s.uploadHost()
	if uploadHost == "" {
		return nil, fmt.Errorf("oss: cannot determine upload host")
	}

	return &common.PresignResult{
		Method: "POST",
		URL:    uploadHost,
		Headers: map[string]string{
			"Authorization": "UpToken " + upToken,
		},
		ExpiresAt: time.Now().Add(expires),
	}, nil
}

// PresignGet 生成私有空间的签名下载 URL。
func (s *Storage) PresignGet(ctx context.Context, bucket, key string, expires time.Duration) (*common.PresignResult, error) {
	if s == nil || s.client == nil {
		return nil, fmt.Errorf("oss: client not initialized")
	}
	if expires <= 0 {
		expires = 10 * time.Minute
	}

	domain := s.client.Domain
	if domain == "" {
		return nil, fmt.Errorf("oss: public endpoint is required for presigned get")
	}

	deadline := time.Now().Add(expires).Unix()
	url := storage.MakePrivateURL(s.client.Mac, domain, key, deadline)

	return &common.PresignResult{
		Method:    "GET",
		URL:       url,
		ExpiresAt: time.Unix(deadline, 0),
	}, nil
}

// Get 服务端下载文件内容。
func (s *Storage) Get(ctx context.Context, bucket, key string) ([]byte, string, error) {
	if s == nil || s.client == nil {
		return nil, "", fmt.Errorf("oss: client not initialized")
	}

	body, contentType, err := s.download(ctx, key)
	if err != nil {
		return nil, "", err
	}
	return body, contentType, nil
}

// Put 服务端直接上传文件内容到七牛云。
func (s *Storage) Put(ctx context.Context, bucket, key string, body []byte, contentType string) error {
	if s == nil || s.client == nil {
		return fmt.Errorf("oss: client not initialized")
	}

	return s.upload(ctx, key, body, contentType)
}

// Head 检查文件是否存在，返回基本信息。
func (s *Storage) Head(ctx context.Context, bucket, key string) (exists bool, size int64, etag string, contentType string, err error) {
	if s == nil || s.client == nil {
		return false, 0, "", "", fmt.Errorf("oss: client not initialized")
	}

	mgr := storage.NewBucketManager(s.client.Mac, s.client.Cfg)
	info, err := mgr.Stat(s.client.Bucket, key)
	if err != nil {
		if isQiniuNotFound(err) {
			return false, 0, "", "", nil
		}
		return false, 0, "", "", err
	}
	return true, info.Fsize, info.Hash, info.MimeType, nil
}

// Delete 删除文件。
func (s *Storage) Delete(ctx context.Context, bucket, key string) error {
	if s == nil || s.client == nil {
		return fmt.Errorf("oss: client not initialized")
	}

	mgr := storage.NewBucketManager(s.client.Mac, s.client.Cfg)
	return mgr.Delete(s.client.Bucket, key)
}

// ---- 内部辅助方法 ----

func (s *Storage) uploadHost() string {
	if s.client.Cfg.Zone != nil {
		if len(s.client.Cfg.Zone.SrcUpHosts) > 0 {
			host := s.client.Cfg.Zone.SrcUpHosts[0]
			if s.client.Cfg.UseHTTPS {
				return "https://" + host
			}
			return "http://" + host
		}
	}

	mac := &auth.Credentials{
		AccessKey: s.client.Mac.AccessKey,
		SecretKey: []byte(s.client.Mac.SecretKey),
	}
	zone, err := storage.GetZone(mac.AccessKey, s.client.Bucket)
	if err != nil || zone == nil {
		return ""
	}
	if len(zone.SrcUpHosts) > 0 {
		host := zone.SrcUpHosts[0]
		if s.client.Cfg.UseHTTPS {
			return "https://" + host
		}
		return "http://" + host
	}
	return ""
}

func (s *Storage) download(ctx context.Context, key string) ([]byte, string, error) {
	domain := s.client.Domain
	if domain == "" {
		return nil, "", fmt.Errorf("oss: public endpoint is required for download")
	}

	deadline := time.Now().Add(30 * time.Second).Unix()
	url := storage.MakePrivateURL(s.client.Mac, domain, key, deadline)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, "", fmt.Errorf("oss: create request: %w", err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, "", fmt.Errorf("oss: download: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, "", common.ErrNotFound
	}
	if resp.StatusCode != http.StatusOK {
		return nil, "", fmt.Errorf("oss: download returned status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, "", err
	}
	return body, resp.Header.Get("Content-Type"), nil
}

func (s *Storage) upload(ctx context.Context, key string, body []byte, contentType string) error {
	putPolicy := storage.PutPolicy{
		Scope:   s.client.Bucket + ":" + key,
		Expires: 600,
	}
	upToken := putPolicy.UploadToken(s.client.Mac)

	formUploader := storage.NewFormUploader(s.client.Cfg)
	ret := &storage.PutRet{}
	extra := &storage.PutExtra{}
	if contentType != "" {
		extra.MimeType = contentType
	}

	return formUploader.Put(ctx, ret, upToken, key, bytes.NewReader(body), int64(len(body)), extra)
}

// isQiniuNotFound 判断是否为文件不存在的错误。
func isQiniuNotFound(err error) bool {
	if err == nil {
		return false
	}
	errStr := err.Error()
	if strings.Contains(errStr, "no such file or directory") ||
		strings.Contains(errStr, "612") ||
		strings.Contains(errStr, "file not found") {
		return true
	}
	return false
}

// ---- 保留的辅助函数 ----

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

func deref(v *string) string {
	if v == nil {
		return ""
	}
	return *v
}
