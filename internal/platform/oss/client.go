package oss

import (
	"fmt"
	"strings"

	"github.com/qiniu/go-sdk/v7/auth"
	"github.com/qiniu/go-sdk/v7/storage"

	appcfg "cixing/internal/config"
)

// Client 封装七牛云 Kodo 的访问凭证和配置。
// 与阿里云不同，七牛云不需要 Internal/Public 双 Client，一个凭证 + Bucket 即可。
type Client struct {
	Mac     *auth.Credentials
	Bucket  string
	Cfg     *storage.Config
	Domain  string // CDN 加速域名
	Region  string // 配置中传入的 region 标识，用于 Zone 兜底
}

// NewClient 根据配置创建七牛云 Client。
func NewClient(cfg appcfg.OSSConfig) (*Client, error) {
	ak := strings.TrimSpace(cfg.AccessKeyID)
	sk := strings.TrimSpace(cfg.AccessKeySecret)
	if ak == "" || sk == "" {
		return nil, fmt.Errorf("oss: access key id and secret are required")
	}

	bucket := strings.TrimSpace(cfg.Bucket)
	if bucket == "" {
		return nil, fmt.Errorf("oss: bucket is required")
	}

	mac := auth.New(ak, sk)
	region := strings.TrimSpace(cfg.Region)

	// 优先用配置指定的 Zone，其次依赖 SDK 自动查询
	zone := zoneFromRegion(region)
	storageCfg := &storage.Config{
		Zone:          zone,
		UseHTTPS:      true,
		UseCdnDomains: false,
	}

	return &Client{
		Mac:    mac,
		Bucket: bucket,
		Cfg:    storageCfg,
		Domain: strings.TrimSpace(cfg.PublicEndpoint),
		Region: region,
	}, nil
}

// zoneFromRegion 将配置中的 region 字符串映射为七牛云 Zone 常量。
// 如果 region 为空或无法识别，返回 nil，后续操作由 SDK 自动查询。
func zoneFromRegion(region string) *storage.Zone {
	switch strings.ToLower(strings.TrimSpace(region)) {
	case "cn-east-1", "z0", "huadong":
		return &storage.ZoneHuadong
	case "cn-north-1", "z1", "huabei":
		return &storage.ZoneHuabei
	case "cn-south-1", "z2", "huanan":
		return &storage.ZoneHuanan
	case "na-0", "na0", "beimei":
		return &storage.ZoneBeimei
	case "ap-southeast-1", "xinjiapo":
		return &storage.ZoneXinjiapo
	default:
		return nil
	}
}
