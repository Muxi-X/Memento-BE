package oss

import (
	"fmt"
	"strings"
	"time"

	osscredentials "github.com/aliyun/alibabacloud-oss-go-sdk-v2/oss/credentials"

	appcfg "cixing/internal/config"
)

const (
	credentialModeStatic               = "static"
	credentialModeECSRAMRole           = "ecs_ram_role"
	credentialModeECSRAMRoleAssumeRole = "ecs_ram_role_assume_role"

	minAssumeRoleSessionDuration = 15 * time.Minute
)

// credential 分发器，根据 credentialMode 创建不同的 CredentialsProvider
// static：直接用 cfg 里的 AccessKeyID 和 AccessKeySecret
// ecs_ram_role：使用 RAM Role 方式获取临时凭证，通过 ECS 机器的元数据服务拿临时凭证
// ecs_ram_role_assume_role：先取 ECS RAM Role 临时凭证，再 AssumeRole 到目标账号角色

func newCredentialsProvider(cfg appcfg.OSSConfig) (osscredentials.CredentialsProvider, error) {
	switch credentialMode(cfg) {
	case credentialModeStatic:
		return osscredentials.NewStaticCredentialsProvider(
			strings.TrimSpace(cfg.AccessKeyID),
			strings.TrimSpace(cfg.AccessKeySecret),
		), nil
	case credentialModeECSRAMRole:
		return newECSRAMRoleCredentialsProvider(cfg)
	case credentialModeECSRAMRoleAssumeRole:
		return newECSRAMRoleAssumeRoleCredentialsProvider(cfg)
	default:
		return nil, fmt.Errorf("oss: unsupported credential mode %q", cfg.CredentialMode)
	}
}

func assumeRoleSessionDurationSeconds(duration time.Duration) (int, error) {
	if duration == 0 {
		return 0, nil
	}
	if duration < minAssumeRoleSessionDuration {
		return 0, fmt.Errorf("oss: assume role session duration must be at least %s", minAssumeRoleSessionDuration)
	}
	return int(duration / time.Second), nil
}
