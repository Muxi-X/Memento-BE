package oss

import (
	"fmt"
	"strings"

	osscredentials "github.com/aliyun/alibabacloud-oss-go-sdk-v2/oss/credentials"

	appcfg "cixing/internal/config"
)

// credential 分发器，根据 credentialMode 创建不同的 CredentialsProvider
// static：直接用 cfg 里的 AccessKeyID 和 AccessKeySecret
// ecs_ram_role：使用 RAM Role 方式获取临时凭证，通过 ECS 机器的元数据服务拿临时凭证

func newCredentialsProvider(cfg appcfg.OSSConfig) (osscredentials.CredentialsProvider, error) {
	switch credentialMode(cfg) {
	case "static":
		return osscredentials.NewStaticCredentialsProvider(
			strings.TrimSpace(cfg.AccessKeyID),
			strings.TrimSpace(cfg.AccessKeySecret),
		), nil
	case "ecs_ram_role":
		return newECSRAMRoleCredentialsProvider(cfg)
	default:
		return nil, fmt.Errorf("oss: unsupported credential mode %q", cfg.CredentialMode)
	}
}
